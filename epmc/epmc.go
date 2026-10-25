// Package epmc is a small client for the Europe PMC REST search API.
package epmc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultBase = "https://www.ebi.ac.uk/europepmc/webservices/rest"

type Client struct {
	HTTP  *http.Client
	Base  string
	UA    string
	Sleep func(time.Duration) // swapped out in tests
}

func New(ua string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, Base: DefaultBase, UA: ua, Sleep: time.Sleep}
}

// Record is the subset of a Europe PMC "core" result that gorxiv prints. Raw keeps the full JSON.
type Record struct {
	ID                   string `json:"id"`
	Source               string `json:"source"`
	PMID                 string `json:"pmid"`
	PMCID                string `json:"pmcid"`
	DOI                  string `json:"doi"`
	Title                string `json:"title"`
	AuthorString         string `json:"authorString"`
	PubYear              string `json:"pubYear"`
	AbstractText         string `json:"abstractText"`
	IsOpenAccess         string `json:"isOpenAccess"`
	InEPMC               string `json:"inEPMC"`
	HasPDF               string `json:"hasPDF"`
	License              string `json:"license"`
	FirstPublicationDate string `json:"firstPublicationDate"`
	PageInfo             string `json:"pageInfo"`
	CitedByCount         int    `json:"citedByCount"`
	VersionNumber        int    `json:"versionNumber"`
	AuthorList           struct {
		Author []Author `json:"author"`
	} `json:"authorList"`
	JournalInfo struct {
		Volume               string `json:"volume"`
		Issue                string `json:"issue"`
		PrintPublicationDate string `json:"printPublicationDate"`
		Journal              struct {
			Title string `json:"title"`
		} `json:"journal"`
	} `json:"journalInfo"`
	BookOrReportDetails struct {
		Publisher string `json:"publisher"`
	} `json:"bookOrReportDetails"`
	KeywordList struct {
		Keyword []string `json:"keyword"`
	} `json:"keywordList"`
	FullTextURLList struct {
		FullTextURL []FullTextURL `json:"fullTextUrl"`
	} `json:"fullTextUrlList"`
	VersionList struct {
		Version []struct {
			ID               string `json:"id"`
			VersionNumber    int    `json:"versionNumber"`
			FirstPublishDate string `json:"firstPublishDate"`
		} `json:"version"`
	} `json:"versionList"`
	Raw json.RawMessage `json:"-"`
}

type Author struct {
	FullName string `json:"fullName"`
	Aff      struct {
		List []struct {
			Affiliation string `json:"affiliation"`
		} `json:"authorAffiliation"`
	} `json:"authorAffiliationDetailsList"`
}

type FullTextURL struct {
	Availability     string `json:"availability"`
	AvailabilityCode string `json:"availabilityCode"`
	DocumentStyle    string `json:"documentStyle"`
	Site             string `json:"site"`
	URL              string `json:"url"`
}

type searchResp struct {
	HitCount       int    `json:"hitCount"`
	NextCursorMark string `json:"nextCursorMark"`
	ErrCode        int    `json:"errCode"`
	ErrMsg         string `json:"errMsg"`
	ResultList     struct {
		Result []json.RawMessage `json:"result"`
	} `json:"resultList"`
}

// Get does a GET with the gorxiv User-Agent, retrying once on 429/5xx (Retry-After seconds, else 2 s).
func (c *Client) Get(ctx context.Context, u string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UA)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if attempt > 0 || (resp.StatusCode != 429 && resp.StatusCode < 500) {
			return resp, nil
		}
		wait := 2 * time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 && s <= 60 {
			wait = time.Duration(s) * time.Second
		}
		resp.Body.Close()
		c.Sleep(wait)
	}
}

// Search runs a Europe PMC query, following cursorMark pages until limit records or the end.
// sort is passed through ("" = relevance). Returns the records and the total hit count.
func (c *Client) Search(ctx context.Context, query, sort string, limit int, core bool) ([]Record, int, error) {
	rt := "lite"
	if core {
		rt = "core"
	}
	var out []Record
	cursor, hits := "*", 0
	for len(out) < limit {
		v := url.Values{"query": {query}, "format": {"json"}, "resultType": {rt},
			"pageSize": {strconv.Itoa(min(limit-len(out), 1000))}, "cursorMark": {cursor}}
		if sort != "" {
			v.Set("sort", sort)
		}
		sr, err := c.searchPage(ctx, c.Base+"/search?"+v.Encode())
		if err != nil {
			return nil, 0, err
		}
		hits = sr.HitCount
		for _, raw := range sr.ResultList.Result {
			var r Record
			if err := json.Unmarshal(raw, &r); err != nil {
				return nil, 0, fmt.Errorf("decode record: %w", err)
			}
			r.Raw = raw
			out = append(out, r)
		}
		if len(sr.ResultList.Result) == 0 || sr.NextCursorMark == "" || sr.NextCursorMark == cursor {
			break
		}
		cursor = sr.NextCursorMark
	}
	return out[:min(len(out), limit)], hits, nil
}

func (c *Client) searchPage(ctx context.Context, u string) (*searchResp, error) {
	for attempt := 0; ; attempt++ {
		resp, err := c.Get(ctx, u)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var sr searchResp
		jerr := json.Unmarshal(body, &sr)
		// Europe PMC sometimes reports a 503 inside a JSON body; give it the same one retry.
		if jerr == nil && sr.ErrCode >= 500 && attempt == 0 {
			c.Sleep(2 * time.Second)
			continue
		}
		switch {
		case jerr == nil && sr.ErrMsg != "":
			return nil, fmt.Errorf("europe pmc: %s (code %d)", sr.ErrMsg, sr.ErrCode)
		case resp.StatusCode != 200:
			return nil, fmt.Errorf("europe pmc: HTTP %d: %.200s", resp.StatusCode, body)
		case jerr != nil:
			return nil, fmt.Errorf("europe pmc: bad JSON: %w", jerr)
		}
		return &sr, nil
	}
}

// Lookup fetches the core record for an ID (see DetectID). n is the number of matching records.
func (c *Client) Lookup(ctx context.Context, id string) (*Record, int, error) {
	kind, norm := DetectID(id)
	q := LookupQuery(kind, norm)
	if q == "" {
		return nil, 0, fmt.Errorf("unrecognised id %q (want PMID, PMCxxx, PPRxxx or DOI)", id)
	}
	recs, n, err := c.Search(ctx, q, "", 1, true)
	if err != nil {
		return nil, 0, err
	}
	if len(recs) == 0 {
		return nil, 0, fmt.Errorf("no Europe PMC record for %s", norm)
	}
	return &recs[0], n, nil
}

var tagRE = regexp.MustCompile(`<[^>]+>`)

// StripTags removes inline HTML (Europe PMC titles/abstracts carry <i>, <h4>, ...) and squeezes spaces.
func StripTags(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}
