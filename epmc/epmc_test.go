package epmc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(url string) *Client {
	c := New("gorxiv/test")
	c.Base, c.Sleep = url, func(time.Duration) {}
	return c
}

// The fixture is a real Europe PMC core response (DOI search for the Maffa et al. preprint).
func TestSearchParsesFixture(t *testing.T) {
	fix, err := os.ReadFile("testdata/search_ppr.json")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 { // first call fails: exercises the one retry
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("User-Agent") != "gorxiv/test" || r.URL.Query().Get("resultType") != "core" {
			t.Errorf("bad request: UA %q, %s", r.Header.Get("User-Agent"), r.URL.RawQuery)
		}
		w.Write(fix)
	}))
	defer srv.Close()

	r, n, err := testClient(srv.URL).Lookup(context.Background(), "https://doi.org/10.64898/2026.08.24.746802")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || r.ID != "PPR1305187" || r.Source != "PPR" || r.BookOrReportDetails.Publisher != "bioRxiv" {
		t.Errorf("got n=%d %s:%s publisher %q", n, r.Source, r.ID, r.BookOrReportDetails.Publisher)
	}
	if len(r.AuthorList.Author) != 17 || r.AuthorList.Author[0].FullName != "Maffa S" {
		t.Errorf("authors: %d, first %+v", len(r.AuthorList.Author), r.AuthorList.Author)
	}
	ft := r.FullTextURLList.FullTextURL
	if len(ft) != 1 || ft[0].DocumentStyle != "doi" || ft[0].AvailabilityCode != "F" {
		t.Errorf("fullTextUrlList: %+v", ft)
	}
	if !strings.HasPrefix(StripTags(r.AbstractText), "Background: CRISPR-mediated") {
		t.Errorf("abstract: %.60q", StripTags(r.AbstractText))
	}
	if !strings.Contains(string(r.Raw), `"hasTextMinedTerms"`) {
		t.Error("Raw should keep the full record")
	}
}

func TestIsPDF(t *testing.T) {
	if !IsPDF([]byte("%PDF-1.4\n...")) || IsPDF([]byte("<!DOCTYPE html>")) || IsPDF(nil) {
		t.Error("IsPDF wrong")
	}
}

// FetchPDF must skip an HTML "PDF" link, never touch subscription links, and save the real PDF atomically.
func TestFetchPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html.pdf":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<!DOCTYPE html><title>Just a moment...</title>"))
		case "/real.pdf":
			w.Write([]byte("%PDF-1.7\nbody"))
		case "/paywall.pdf":
			t.Error("subscription link fetched")
		}
	}))
	defer srv.Close()
	r := &Record{}
	r.FullTextURLList.FullTextURL = []FullTextURL{
		{AvailabilityCode: "S", DocumentStyle: "pdf", URL: srv.URL + "/paywall.pdf"},
		{AvailabilityCode: "OA", DocumentStyle: "pdf", URL: srv.URL + "/html.pdf"},
		{AvailabilityCode: "OA", DocumentStyle: "pdf", URL: srv.URL + "/real.pdf"},
	}
	dst := filepath.Join(t.TempDir(), "x.pdf")
	s, tried, err := testClient(srv.URL).FetchPDF(context.Background(), r, dst)
	if err != nil {
		t.Fatal(err, tried)
	}
	if len(tried) != 2 || !strings.Contains(tried[0].Err, "not a PDF") || s.Size != 13 {
		t.Errorf("tried %+v, saved %+v", tried, s)
	}
	if b, _ := os.ReadFile(dst); string(b) != "%PDF-1.7\nbody" {
		t.Errorf("file = %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), ".gorxiv-*")); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}
