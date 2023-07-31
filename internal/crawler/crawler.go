package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/bertinatto/testgrid/internal"
	"github.com/gocolly/colly"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Crawler struct {
	org           string
	repo          string
	pullRequestID int
	data          map[string][]*internal.ProwJob
	collector     *colly.Collector
}

func New(org, repo string, prID int) *Crawler {
	allowedDomains := []string{
		"github.com",
		"api.github.com",
		"pr-payload-tests.ci.openshift.org",
		"gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com",
	}
	return &Crawler{
		org:           org,
		repo:          repo,
		pullRequestID: prID,
		data:          make(map[string][]*internal.ProwJob, 128),
		collector:     newCollector(allowedDomains...),
	}
}

func (c *Crawler) Do() map[string][]*internal.ProwJob {
	urls := c.parsePR()
	prowJobsURLs, finishedURLs := c.parsePayloadJobs(urls)
	installURLs := c.parseProwJobsURLs(prowJobsURLs)
	c.parseInstallTXT(installURLs)
	c.parseFinishedJSON(finishedURLs)
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
	collector.Visit(fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=50", c.org, c.repo, c.pullRequestID))

	return payloadJobs.List()
}

func (c *Crawler) parsePayloadJobs(urls []string) ([]string, []string) {
	prowJobsURLs := []string{}
	finishedURLs := []string{}
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

			prowJobsURLs = append(prowJobsURLs, href)
			finishedURLs = append(finishedURLs, finished.String())
		})
	})

	// Finally, visit all urls provided to this function.
	for _, url := range urls {
		collector.Visit(url)
	}

	return prowJobsURLs, finishedURLs
}

func (c *Crawler) parseFinishedJSON(urls []string) {
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

func (c *Crawler) parseInstallTXT(urls []string) {
	collector := newCollector("gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com")

	// Before visiting prow job pages, create a callback that will be called for every visited page.
	collector.OnResponse(func(r *colly.Response) {
		status := string(bytes.TrimSpace(r.Body))
		_, err := strconv.Atoi(status)
		if err != nil {
			// This means the status is invalid, so leave it empty
			return
		}

		// Assume the installation succeeded if the install-status.txt
		// file contains "0" otherwise assume that the it failed.
		switch status {
		case "":
			break
		case "0":
			status = "success"
		default:
			status = "failure"
		}

		// Store the installation status to our global store.
		for _, values := range c.data {
			for _, j := range values {
				if j.InstallStatusURL == r.Request.URL.String() {
					j.InstallStatus = string(status)
				}
			}
		}
	})

	// Finally, Visit all prow job urls provided to this function.
	for _, url := range urls {
		collector.Visit(url)
	}
}

func (c *Crawler) parseProwJobsURLs(urls []string) []string {
	installURLs := []string{}
	collector := newCollector("prow.ci.openshift.org")

	// Before visiting prow job pages, create a callback that will be called for every visited page.
	collector.OnResponse(func(r *colly.Response) {
		lensArtifacts := map[string][]string{}
		re := regexp.MustCompile(`var lensArtifacts = (.+?);`)
		matches := re.FindSubmatch(r.Body)
		if len(matches) > 1 {
			jsonStr := matches[1]
			err := json.Unmarshal([]byte(jsonStr), &lensArtifacts)
			if err != nil {
				log.Fatalf("error unmarshalling: %v", err)
			}
		}

		statusPath := ""
		for _, v := range lensArtifacts["0"] {
			if strings.HasSuffix(v, "gather-must-gather/finished.json") {
				statusPath = strings.ReplaceAll(v, "finished.json", "artifacts/install-status.txt")
			}
		}

		if statusPath != "" {
			// Construct the URL for the install-status.txt file.
			base := strings.ReplaceAll(
				r.Request.URL.String(),
				"https://prow.ci.openshift.org/view/gs/",
				"https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/",
			)
			install, err := url.Parse(base)
			if err != nil {
				log.Fatalf("error parsing %q: %v", base, err)
			}
			install.Path = path.Join(install.Path, statusPath)
			installURLs = append(installURLs, install.String())

			// Store the install status URL to our global store.
			for _, values := range c.data {
				for _, j := range values {
					if j.URL == r.Request.URL.String() {
						j.InstallStatusURL = install.String()
					}
				}
			}
		}
	})

	// Finally, Visit all prow job urls provided to this function.
	for _, url := range urls {
		collector.Visit(url)
	}

	return installURLs
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
