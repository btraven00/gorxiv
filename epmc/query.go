package epmc

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Filters are convenience flags ANDed onto a raw Europe PMC query.
type Filters struct {
	Preprints bool   // SRC:PPR
	OA        bool   // OPEN_ACCESS:y
	From, To  string // YYYY, YYYY-MM or YYYY-MM-DD, on FIRST_PDATE
	Author    string // AUTH:"..."
}

// Compose returns `(q) AND filter AND ...`; q is passed through unchanged.
func Compose(q string, f Filters) (string, error) {
	q = strings.TrimSpace(q)
	var parts []string
	if q != "" {
		parts = append(parts, q)
	}
	if f.Author != "" {
		parts = append(parts, `AUTH:"`+strings.ReplaceAll(f.Author, `"`, ``)+`"`)
	}
	if f.Preprints {
		parts = append(parts, "SRC:PPR")
	}
	if f.OA {
		parts = append(parts, "OPEN_ACCESS:y")
	}
	if f.From != "" || f.To != "" {
		from, to := "*", "*"
		var err error
		if f.From != "" {
			if from, err = dateBound(f.From, false); err != nil {
				return "", err
			}
		}
		if f.To != "" {
			if to, err = dateBound(f.To, true); err != nil {
				return "", err
			}
		}
		parts = append(parts, fmt.Sprintf("FIRST_PDATE:[%s TO %s]", from, to))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty query: give search terms or a filter")
	}
	if len(parts) > 1 && q != "" {
		parts[0] = "(" + q + ")"
	}
	return strings.Join(parts, " AND "), nil
}

// dateBound expands YYYY / YYYY-MM / YYYY-MM-DD to the first (or, if end, last) day it covers.
func dateBound(s string, end bool) (string, error) {
	for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
		t, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		if end {
			switch layout {
			case "2006-01":
				t = t.AddDate(0, 1, -1)
			case "2006":
				t = t.AddDate(1, 0, -1)
			}
		}
		return t.Format("2006-01-02"), nil
	}
	return "", fmt.Errorf("bad date %q (want YYYY, YYYY-MM or YYYY-MM-DD)", s)
}

// SortParam maps a --sort name to Europe PMC's sort parameter.
func SortParam(s string) (string, error) {
	switch s {
	case "", "relevance":
		return "", nil
	case "date":
		return "FIRST_PDATE_D desc", nil
	case "cited":
		return "CITED desc", nil
	}
	return "", fmt.Errorf("bad --sort %q (relevance, date, cited)", s)
}

type Kind int

const (
	Unknown Kind = iota
	PMID
	PMC
	PPR
	DOI
)

var (
	digitsRE = regexp.MustCompile(`^\d+$`)
	pmcRE    = regexp.MustCompile(`^(?i)PMC\d+$`)
	pprRE    = regexp.MustCompile(`^(?i)PPR\d+$`)
	doiRE    = regexp.MustCompile(`^10\.\d{4,9}/\S+$`)
)

// DetectID classifies an identifier and returns it normalised (upper-case PMC/PPR, bare DOI).
func DetectID(s string) (Kind, string) {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "doi.org/", "doi:"} {
		if strings.HasPrefix(low, p) {
			s, low = s[len(p):], low[len(p):]
			break
		}
	}
	if strings.HasPrefix(low, "pmid:") {
		s = strings.TrimSpace(s[5:])
	}
	switch {
	case digitsRE.MatchString(s):
		return PMID, s
	case pmcRE.MatchString(s):
		return PMC, strings.ToUpper(s)
	case pprRE.MatchString(s):
		return PPR, strings.ToUpper(s)
	case doiRE.MatchString(s):
		return DOI, s
	}
	return Unknown, s
}

// LookupQuery is the Europe PMC query that finds one ID ("" if the kind is unknown).
func LookupQuery(k Kind, id string) string {
	switch k {
	case PMID:
		return "EXT_ID:" + id + " AND SRC:MED"
	case PMC:
		return "PMCID:" + id
	case PPR:
		return "EXT_ID:" + id + " AND SRC:PPR"
	case DOI:
		return `DOI:"` + id + `"`
	}
	return ""
}
