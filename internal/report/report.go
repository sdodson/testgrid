package report

import (
	"fmt"
	"html/template"
	"log"
	"os"

	"github.com/bertinatto/testgrid/html"
	"github.com/bertinatto/testgrid/internal"
	"github.com/bertinatto/testgrid/variants/generated"
)

type Report struct {
	tmpl   *template.Template
	matrix map[string]internal.Entry
}

func New() *Report {
	return &Report{
		matrix: make(map[string]internal.Entry, 128),
		tmpl:   template.Must(template.New("").ParseFS(html.FS, "*.tmpl")),
	}
}

func (r *Report) Create(jobs map[string][]*internal.ProwJob) error {
	if len(jobs) == 0 {
		return fmt.Errorf("no jobs to create report")
	}
	for _, v := range jobs {
		for _, pj := range v {
			currentVariant, ok := generated.Variants[pj.Name]
			if !ok {
				log.Printf("WARNING: Job %q does not have a known variant\n", pj.Name)
				continue
			}

			if e, ok := r.matrix[currentVariant.Name]; !ok {
				// Matrix doesn't have this variant yet: just add it
				r.matrix[currentVariant.Name] = newEntry(&currentVariant, pj)

			} else {
				// Entry already exists in matrix, just update it with the PASSING jobs
				r.matrix[currentVariant.Name] = updateEntry(&e, &currentVariant, pj)
			}
		}
	}
	return nil
}

func (r *Report) WriteToFile(file string) error {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to open report file: %w", err)
	}
	defer f.Close()

	err = r.tmpl.ExecuteTemplate(f, "matrix", r.matrix)
	if err != nil {
		return fmt.Errorf("failed to execute template 'matrix': %w", err)
	}
	return nil
}

func updateEntry(e *internal.Entry, v *internal.Variant, p *internal.ProwJob) internal.Entry {
	newEntry := *e
	c := internal.Cell{URL: p.URL, Result: p.Result}
	if e.InstallSuccess.Result != "success" && p.InstallStatus == "success" {
		newEntry.InstallSuccess = internal.Cell{URL: p.InstallStatusURL, Result: p.InstallStatus}
	}
	if v.Parallel {
		if e.Parallel.Result != "success" && p.Result == "success" {
			newEntry.Parallel = c
		}
	}
	if v.Serial {
		if e.Serial.Result != "success" && p.Result == "success" {
			newEntry.Serial = c
		}
	}
	if v.CSI {
		if e.CSI.Result != "success" && p.Result == "success" {
			newEntry.CSI = c
		}
	}
	if v.UpgradeFromCurrent {
		if e.UpgradeFromCurrent.Result != "success" && p.Result == "success" {
			newEntry.UpgradeFromCurrent = c
		}
	}
	if v.UpgradeFromPrevious {
		if e.UpgradeFromPrevious.Result != "success" && p.Result == "success" {
			newEntry.UpgradeFromPrevious = c
		}
	}
	return newEntry
}

func newEntry(v *internal.Variant, p *internal.ProwJob) internal.Entry {
	c := internal.Cell{URL: p.URL, Result: p.Result}
	e := internal.Entry{Variant: v.Name}
	e.InstallSuccess = internal.Cell{URL: p.InstallStatusURL, Result: p.InstallStatus}
	if v.Parallel {
		e.Parallel = c
	}
	if v.Serial {
		e.Serial = c
	}
	if v.CSI {
		e.CSI = c
	}
	if v.UpgradeFromCurrent {
		e.UpgradeFromCurrent = c
	}
	if v.UpgradeFromPrevious {
		e.UpgradeFromPrevious = c
	}
	return e
}
