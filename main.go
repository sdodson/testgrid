//go:generate go run ./variants/main.go -input ./variants/input.tsv -output ./variants/generated/zz_generated.variants.go

package main

import (
	"log"
	"os"
	"testgrid/internal"
	"testgrid/variants/generated"
)

var matrix = map[string]internal.Entry{}

func main() {
	jobs := NewCrawler(1558).Do()

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
