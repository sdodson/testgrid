//go:generate go run ./variants/main.go -input ./variants/input.tsv -output ./variants/generated/zz_generated.variants.go

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"

	"github.com/bertinatto/testgrid/internal"
	"github.com/bertinatto/testgrid/variants/generated"
)

var matrix = map[string]internal.Entry{}

func main() {
	prFlag := flag.String("pr", "", "pull request in the format 'org/repo#prID'")
	flag.Parse()

	// Validate the input string
	validInput, err := regexp.MatchString(`^(\w+)/(\w+)#(\d+)$`, *prFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to validate input: %v\n", err)
		os.Exit(1)
	}

	if !validInput {
		fmt.Fprintf(os.Stderr, "Invalid input format. Expected: 'org/repo#pr: %v\n", err)
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Extract organization, repository, and pull request ID
	re := regexp.MustCompile(`^(\w+)/(\w+)#(\d+)$`)
	matches := re.FindStringSubmatch(*prFlag)

	org := matches[1]
	repo := matches[2]
	prID, err := strconv.Atoi(matches[3])
	if err != nil {
		fmt.Println("Failed to convert pull request ID to integer:", err)
		os.Exit(1)
	}

	jobs := NewCrawler(org, repo, prID).Do()

	for _, v := range jobs {
		for _, pj := range v {
			currentVariant, ok := generated.Variants[pj.Name]
			if !ok {
				// Unknown variant, can't process this prow job.
				// TODO: log so that we can update store of variants later
				continue
			}

			if e, ok := matrix[currentVariant.Name]; !ok {
				// Matrix doesn't have this variant yet: just add it
				matrix[currentVariant.Name] = newEntry(&currentVariant, pj)

			} else {
				// Entry already exists in matrix, just update it with the PASSING jobs
				matrix[currentVariant.Name] = updateEntry(&e, &currentVariant, pj)
			}
		}
	}

	f, err := os.OpenFile("report.html", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	err = tmpl.ExecuteTemplate(f, "matrix", matrix)
	if err != nil {
		log.Fatal(err)
	}
}
