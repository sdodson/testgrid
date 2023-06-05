package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strings"
	"testgrid/internal"

	"github.com/gocolly/colly"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Crawler struct {
	pullRequestID int
	data          map[string][]*internal.ProwJob
	collector     *colly.Collector
}

func NewCrawler(id int) *Crawler {
	allowedDomains := []string{
		"github.com",
		"api.github.com",
		"pr-payload-tests.ci.openshift.org",
		"gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com",
	}
	return &Crawler{
		pullRequestID: id,
		data:          make(map[string][]*internal.ProwJob, 128),
		collector:     newCollector(allowedDomains...),
	}
}

func (c *Crawler) Do() map[string][]*internal.ProwJob {
	urls := c.parsePR()
	prowJobs := c.parsePayloadJobs(urls)
	c.parseProwJobs(prowJobs)
	return c.data
}

func (c *Crawler) parsePR() []string {
	payloadJobs := sets.NewString()
	collector := newCollector("github.com", "api.github.com")

	// Create a callback that will be called once we visit the PR page.
	collector.OnResponse(func(r *colly.Response) {
		var comments []struct {
			URL  string `string:"url"`
			Body string `string:"body"`
		}

		if err := json.Unmarshal(r.Body, &comments); err != nil {
			log.Fatalf("error unmarshalling %q: %v", r.Request.URL.String(), err)
		}

		// Create a list of all links to payload runs available in the PR page.
		urls := []string{}
		re := regexp.MustCompile(`https://pr-payload-tests\.ci\.openshift\.org/runs/ci/.+`)
		for _, c := range comments {
			urls = append(urls, re.FindAllString(c.Body, -1)...)
		}

		// Filter out and deduplicate urls found.
		for _, url := range urls {
			if strings.HasPrefix(url, "https://pr-payload-tests.ci.openshift.org") {
				payloadJobs.Insert(url)
			}
		}
	})

	// Finally, visit the PR page (through the API).
	collector.Visit(fmt.Sprintf("https://api.github.com/repos/openshift/kubernetes/issues/%d/comments", c.pullRequestID))

	return payloadJobs.List()
}

func (c *Crawler) parsePayloadJobs(urls []string) []string {
	prowJobURLs := []string{}
	collector := newCollector("pr-payload-tests.ci.openshift.org")

	// Create a callback that will be called once we visit payload job page.
	collector.OnHTML("li", func(e *colly.HTMLElement) {
		e.ForEach("tt", func(_ int, el *colly.HTMLElement) {
			jobName := el.ChildText("span")
			href := el.ChildAttr("a", "href")

			// We are only interested in prow jobs URLs.
			if !strings.HasPrefix(href, "https://prow.ci.openshift.org/") {
				return
			}

			// Construct the URL for the finished.json file.
			base := strings.ReplaceAll(
				href,
				"https://prow.ci.openshift.org/view/gs/",
				"https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/",
			)
			finished, err := url.Parse(base)
			if err != nil {
				log.Fatalf("error parsing %q: %v", base, err)
			}
			finished.Path = path.Join(finished.Path, "finished.json")

			// Store what we have found so  far. We'll fetch and parse the finished.json file later on.
			c.data[jobName] = append(c.data[jobName], &internal.ProwJob{
				Name:      jobName,
				URL:       href,
				ResultURL: finished.String(),
			})

			prowJobURLs = append(prowJobURLs, finished.String())
		})
	})

	// Finally, visit all urls provided to this function.
	for _, url := range urls {
		collector.Visit(url)
	}

	return prowJobURLs
}

func (c *Crawler) parseProwJobs(urls []string) {
	collector := newCollector("gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com")

	// Before visiting prow job pages, create a callback that will be called for every visited page.
	collector.OnResponse(func(r *colly.Response) {
		jobResult := map[string]any{}
		if err := json.Unmarshal(r.Body, &jobResult); err != nil {
			log.Fatalf("error unmarshalling %q: %v", r.Request.URL.String(), err)
		}

		result := strings.ToLower(jobResult["result"].(string))

		// Store the result to our global store.
		for _, values := range c.data {
			for _, j := range values {
				if j.ResultURL == r.Request.URL.String() {
					j.Result = result
				}
			}
		}
	})

	// Finally, Visit all prow job urls provided to this function.
	for _, url := range urls {
		collector.Visit(url)
	}
}

func newCollector(allowed ...string) *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(allowed...),
		// FIXME: add mechanism to disable
		colly.CacheDir("/tmp/testgrid_cache"),
	)

	// Create a callback that will run before every request made.
	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting", r.URL.String())
	})

	return c
}
