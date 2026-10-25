package epmc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PMCOpenData is NCBI's PMC Article Datasets bucket (open-access + author-manuscript subsets).
var PMCOpenData = "https://pmc-oa-opendata.s3.amazonaws.com"

// Attempt is one URL tried by FetchPDF and why it failed ("" = success).
type Attempt struct{ URL, Err string }

type Saved struct {
	Path, URL, SHA256 string
	Size              int64
}

// IsPDF reports whether the first bytes are the PDF magic.
func IsPDF(head []byte) bool { return bytes.HasPrefix(head, []byte("%PDF-")) }

var rxivRE = regexp.MustCompile(`^https://(www\.)?(bio|med)rxiv\.org/content/`)

// FetchPDF tries the openly available full-text sources of r in order and saves the first real PDF
// to dst (temp file + rename). Order: Europe PMC render, PMC open-data bucket (PMCID only), then
// fullTextUrlList pdf links, then free/OA DOI links (bioRxiv/medRxiv landing -> .full.pdf).
// Subscription-only links are never tried.
func (c *Client) FetchPDF(ctx context.Context, r *Record, dst string) (*Saved, []Attempt, error) {
	var tried []Attempt
	seen := map[string]bool{}
	try := func(u string) (*Saved, string) {
		if seen[u] {
			return nil, ""
		}
		seen[u] = true
		s, final, err := c.savePDF(ctx, u, dst)
		if err != nil {
			tried = append(tried, Attempt{u, err.Error()})
			return nil, final
		}
		tried = append(tried, Attempt{u, ""})
		return s, final
	}
	if r.PMCID != "" {
		if s, _ := try("https://europepmc.org/articles/" + r.PMCID + "?pdf=render"); s != nil {
			return s, tried, nil
		}
		if u, err := c.pmcOpenDataPDF(ctx, r.PMCID); err != nil {
			tried = append(tried, Attempt{PMCOpenData + "/" + r.PMCID + ".*", err.Error()})
		} else if s, _ := try(u); s != nil {
			return s, tried, nil
		}
	}
	ft := r.FullTextURLList.FullTextURL
	for _, f := range ft {
		if f.DocumentStyle == "pdf" && f.AvailabilityCode != "S" {
			if s, _ := try(f.URL); s != nil {
				return s, tried, nil
			}
		}
	}
	for _, f := range ft {
		if f.DocumentStyle != "doi" || (f.AvailabilityCode != "OA" && f.AvailabilityCode != "F") {
			continue
		}
		s, final := try(f.URL)
		if s != nil {
			return s, tried, nil
		}
		if rxivRE.MatchString(final) {
			if s, _ := try(strings.TrimSuffix(final, ".full") + ".full.pdf"); s != nil {
				return s, tried, nil
			}
		}
	}
	return nil, tried, fmt.Errorf("no openly available PDF found (%d URLs tried)", len(tried))
}

// savePDF GETs u and, if the body starts with %PDF-, writes it atomically to dst.
// It returns the final URL after redirects, also on failure.
func (c *Client) savePDF(ctx context.Context, u, dst string) (*Saved, string, error) {
	resp, err := c.Get(ctx, u)
	if err != nil {
		return nil, u, err
	}
	defer resp.Body.Close()
	final := resp.Request.URL.String()
	ct := resp.Header.Get("Content-Type")
	br := bufio.NewReader(resp.Body)
	head, _ := br.Peek(512)
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if resp.Header.Get("Cf-Mitigated") == "challenge" || bytes.Contains(head, []byte("Just a moment")) {
			msg += " (bot-challenge page; not circumvented)"
		}
		return nil, final, fmt.Errorf("%s", msg)
	}
	if !IsPDF(head) {
		return nil, final, fmt.Errorf("not a PDF (Content-Type %q)", ct)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".gorxiv-*.part")
	if err != nil {
		return nil, final, err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), br)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644) // CreateTemp makes 0600
	}
	if err != nil {
		return nil, final, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return nil, final, err
	}
	return &Saved{Path: dst, URL: final, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, final, nil
}

// pmcOpenDataPDF finds the latest-version PDF key for pmcid in the PMC open-data bucket.
func (c *Client) pmcOpenDataPDF(ctx context.Context, pmcid string) (string, error) {
	resp, err := c.Get(ctx, PMCOpenData+"/?list-type=2&prefix="+url.QueryEscape(pmcid+"."))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bucket listing: HTTP %d", resp.StatusCode)
	}
	var l struct {
		Contents []struct{ Key string } `xml:"Contents"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&l); err != nil {
		return "", fmt.Errorf("bucket listing: %w", err)
	}
	best, bestV := "", -1
	for _, o := range l.Contents {
		dir, file, ok := strings.Cut(o.Key, "/")
		if !ok || file != dir+".pdf" {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimPrefix(dir, pmcid+".")); err == nil && v > bestV {
			best, bestV = o.Key, v
		}
	}
	if best == "" {
		return "", fmt.Errorf("not in the PMC open-access/author-manuscript dataset (or no PDF there)")
	}
	return PMCOpenData + "/" + best, nil
}
