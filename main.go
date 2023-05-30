package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

// prowJob represents the result for a Prow job run.
type prowJob struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	ResultURL string `json:"result_file"`
	Result    string `json:"result"`
}

type Comment struct {
	URL  string `string:"url"`
	Body string `string:"body"`
}

// jobs is a global store for the information collected.
var jobs = map[string][]*prowJob{}

func main() {
	ghCollector := colly.NewCollector(
		colly.AllowedDomains("github.com", "api.github.com", "pr-payload-tests.ci.openshift.org", "gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com"),
		// FIXME: remove
		colly.CacheDir("./testgrid_cache"),
	)
	payloadJobCollector := ghCollector.Clone()
	prowJobCollector := ghCollector.Clone()

	ghCollector.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})

	payloadJobCollector.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})
	prowJobCollector.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})

	// Grab all payload job runs from the PR page.
	urls := map[string]struct{}{}
	ghCollector.OnResponse(func(r *colly.Response) {
		var comments []Comment
		if err := json.Unmarshal(r.Body, &comments); err != nil {
			log.Fatalf("error unmarshalling %q: %v", r.Request.URL.String(), err)
		}

		// Create a list of all links to payload runs available in the PR page.
		var payloadJobsURLs []string
		re := regexp.MustCompile(`https://pr-payload-tests\.ci\.openshift\.org/runs/ci/.+`)
		for _, c := range comments {
			payloadJobsURLs = append(payloadJobsURLs, re.FindAllString(c.Body, -1)...)
		}

		// Deduplicate.
		for _, url := range payloadJobsURLs {
			if strings.HasPrefix(url, "https://pr-payload-tests.ci.openshift.org") {
				urls[url] = struct{}{}
			}
		}

		// Visit each payload job page and let the respective collector do its work.
		for url := range urls {
			payloadJobCollector.Visit(url)
		}
	})

	payloadJobCollector.OnHTML("li", func(e *colly.HTMLElement) {
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
			jobs[jobName] = append(jobs[jobName], &prowJob{
				Name:      jobName,
				URL:       href,
				ResultURL: finished.String(),
			})
			prowJobCollector.Visit(finished.String())
		})
	})

	prowJobCollector.OnResponse(func(r *colly.Response) {
		jobResult := map[string]any{}
		if err := json.Unmarshal(r.Body, &jobResult); err != nil {
			log.Fatalf("error unmarshalling %q: %v", r.Request.URL.String(), err)
		}

		result := strings.ToLower(jobResult["result"].(string))

		// Store the result to our global store.
		for _, values := range jobs {
			for _, j := range values {
				if j.ResultURL == r.Request.URL.String() {
					j.Result = result
				}
			}
		}
	})

	// ghCollector.Visit("https://github.com/openshift/kubernetes/pull/1558")
	ghCollector.Visit("https://api.github.com/repos/openshift/kubernetes/issues/1558/comments")

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	// enc.Encode(urls)
	enc.Encode(jobs)

	for _, v := range jobs {
		for _, pj := range v {
			currentVariant, ok := variants[pj.Name]
			if !ok {
				// Unknown variant, can't process this prow job
				continue
			}

			if e, ok := matrix[currentVariant.Name]; !ok {
				// Matrix doesn't have this variant yet
				matrix[currentVariant.Name] = entry{
					Variant:   currentVariant.Name,
					E2ESerial: map[string]string{pj.URL: pj.Result},
				}
			} else {
				// Entry already exists in matrix
				if pj.Result == "success" {
					e.
				}
			}

		}
	}
}

// map[string]bool means "prow job link" -> passed (true/false)
type entry struct {
	Variant             string
	InstallSuccess      map[string]bool
	OverallTest         bool
	UpgradeFromCurrent  map[string]bool
	UpgradeFromPrevious map[string]bool
	E2ESerial           map[string]string
	E2EParallel         map[string]bool
	CSI                 map[string]bool
}

// var variants = map[string]string{
//
// }

type variant struct {
	Name     string
	Serial   bool
	Parallel bool
	CSI      bool
}

var variants = map[string]variant{
	"periodic-ci-openshift-release-master-nightly-4.14-e2e-aws-csi": {
		Name:     "aws,ovn,ha",
		Serial:   false,
		Parallel: true,
		CSI:      true,
	},
}

// aws,ovn,ha -> entry
var matrix = map[string]entry{}

// 1. go through every map entry:

// "periodic-ci-openshift-release-master-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade": [
//  {
//   "name": "periodic-ci-openshift-release-master-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade",
//   "url": "https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/openshift-kubernetes-1558-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade/1651934220504797184",
//   "result_file": "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/logs/openshift-kubernetes-1558-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade/1651934220504797184/finished.json",
//   "result": "success"
//  },
//  {
//   "name": "periodic-ci-openshift-release-master-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade",
//   "url": "https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/openshift-kubernetes-1558-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade/1651674819680276480",
//   "result_file": "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/logs/openshift-kubernetes-1558-nightly-4.14-upgrade-from-stable-4.13-e2e-metal-ipi-sdn-bm-upgrade/1651674819680276480/finished.json",
//   "result": "success"
//  }
// ],

// 2. get variant struct for that job using "variants"
// 3. store that variant in "matrix" map, avoiding overriding key that succeeded
// 4. pass matrix to golang template
