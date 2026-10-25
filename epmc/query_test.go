package epmc

import "testing"

func TestCompose(t *testing.T) {
	cases := []struct {
		q    string
		f    Filters
		want string
	}{
		{"perturb-seq", Filters{}, "perturb-seq"},
		{"a OR b", Filters{Preprints: true, OA: true}, "(a OR b) AND SRC:PPR AND OPEN_ACCESS:y"},
		{"x", Filters{From: "2020", To: "2021"}, "(x) AND FIRST_PDATE:[2020-01-01 TO 2021-12-31]"},
		{"x", Filters{To: "2024-02"}, "(x) AND FIRST_PDATE:[* TO 2024-02-29]"},
		{"x", Filters{From: "2023-05-06"}, "(x) AND FIRST_PDATE:[2023-05-06 TO *]"},
		{"", Filters{Author: "Dempster JM"}, `AUTH:"Dempster JM"`},
	}
	for _, c := range cases {
		got, err := Compose(c.q, c.f)
		if err != nil || got != c.want {
			t.Errorf("Compose(%q, %+v) = %q, %v; want %q", c.q, c.f, got, err, c.want)
		}
	}
	if _, err := Compose("", Filters{}); err == nil {
		t.Error("empty query should fail")
	}
	if _, err := Compose("x", Filters{From: "last year"}); err == nil {
		t.Error("bad date should fail")
	}
}

func TestDetectID(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		norm string
	}{
		{"32855387", PMID, "32855387"},
		{"PMID:32855387", PMID, "32855387"},
		{"pmc7453022", PMC, "PMC7453022"},
		{"PPR1305187", PPR, "PPR1305187"},
		{"10.64898/2026.08.24.746802", DOI, "10.64898/2026.08.24.746802"},
		{"https://doi.org/10.1038/s41467-020-17440-w", DOI, "10.1038/s41467-020-17440-w"},
		{"doi:10.1101/2020.01.01.123456", DOI, "10.1101/2020.01.01.123456"},
		{"hello", Unknown, "hello"},
	}
	for _, c := range cases {
		k, n := DetectID(c.in)
		if k != c.kind || n != c.norm {
			t.Errorf("DetectID(%q) = %v %q; want %v %q", c.in, k, n, c.kind, c.norm)
		}
	}
	if q := LookupQuery(DOI, "10.1/x"); q != `DOI:"10.1/x"` {
		t.Errorf("LookupQuery DOI = %q", q)
	}
}
