package main

import (
	"html/template"
	"testgrid/html"
	"testgrid/internal"
)

var tmpl = template.Must(template.New("").ParseFS(html.FS, "*.tmpl"))

func updateEntry(e *internal.Entry, v *internal.Variant, p *internal.ProwJob) internal.Entry {
	c := internal.Cell{URL: p.URL, Result: p.Result}
	newEntry := *e
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
