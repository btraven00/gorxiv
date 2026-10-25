package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/btraven00/gorxiv/epmc"
)

var (
	sf          epmc.Filters
	sSort       string
	sLimit      int
	sJSON, sIDs bool
	sPrintQuery bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Europe PMC (query syntax passed through unchanged)",
	Example: `  gorxiv search 'perturb-seq AND "cancer cell lines"' --preprints --from 2025 -n 10
  gorxiv search --author "Dempster JM" --sort date
  gorxiv search 'TITLE:"mix-seq"' --oa --ids | xargs -n1 gorxiv pdf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := epmc.Compose(strings.Join(args, " "), sf)
		if err != nil {
			return err
		}
		sort, err := epmc.SortParam(sSort)
		if err != nil {
			return err
		}
		if sPrintQuery {
			fmt.Fprintln(os.Stderr, "query:", q)
		}
		recs, hits, err := client().Search(context.Background(), q, sort, sLimit, sJSON)
		if err != nil {
			return err
		}
		switch {
		case sJSON:
			raws := make([]string, len(recs))
			for i, r := range recs {
				raws[i] = string(r.Raw)
			}
			fmt.Println("[" + strings.Join(raws, ",\n") + "]")
		case sIDs:
			for _, r := range recs {
				fmt.Println(shortID(&r))
			}
		default:
			printTable(recs)
			fmt.Fprintf(os.Stderr, "%d of %d hits\n", len(recs), hits)
		}
		return nil
	},
}

func init() {
	f := searchCmd.Flags()
	f.BoolVar(&sf.Preprints, "preprints", false, "preprints only (SRC:PPR)")
	f.BoolVar(&sf.OA, "oa", false, "open access only (OPEN_ACCESS:y)")
	f.StringVar(&sf.From, "from", "", "first published on/after (YYYY, YYYY-MM or YYYY-MM-DD)")
	f.StringVar(&sf.To, "to", "", "first published on/before (YYYY, YYYY-MM or YYYY-MM-DD)")
	f.StringVar(&sf.Author, "author", "", `author, Europe PMC style ("Dempster JM" or "Dempster")`)
	f.StringVar(&sSort, "sort", "relevance", "relevance, date (newest first) or cited")
	f.IntVarP(&sLimit, "limit", "n", 20, "max records (pages through results)")
	f.BoolVar(&sJSON, "json", false, "print raw core records as a JSON array")
	f.BoolVar(&sIDs, "ids", false, "print bare IDs, one per line (accepted by show/pdf)")
	f.BoolVar(&sPrintQuery, "print-query", false, "print the composed query to stderr")
	rootCmd.AddCommand(searchCmd)
}

// shortID is the ID that show/pdf accept: PMCID if any, else PMID / PPR id, else source:id.
func shortID(r *epmc.Record) string {
	switch {
	case r.PMCID != "":
		return r.PMCID
	case r.Source == "MED" || r.Source == "PPR":
		return r.ID
	case r.DOI != "":
		return r.DOI
	}
	return r.Source + ":" + r.ID
}

func firstAuthor(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	first, rest, _ := strings.Cut(s, ",")
	if strings.TrimSpace(rest) != "" {
		first += " et al."
	}
	return first
}

func flag(r *epmc.Record) string {
	switch {
	case r.IsOpenAccess == "Y" && r.HasPDF == "Y":
		return "oa+pdf"
	case r.IsOpenAccess == "Y":
		return "oa"
	case r.HasPDF == "Y":
		return "pdf"
	}
	return "-"
}

func printTable(recs []epmc.Record) {
	type row struct{ id, src, year, au, title, doi, flag string }
	rows := make([]row, len(recs))
	wID, wAu, wDOI := 2, 6, 3
	for i := range recs {
		r := &recs[i]
		rows[i] = row{shortID(r), r.Source, r.PubYear, trunc(firstAuthor(r.AuthorString), 22),
			epmc.StripTags(r.Title), r.DOI, flag(r)}
		wID = max(wID, utf8.RuneCountInString(rows[i].id))
		wAu = max(wAu, utf8.RuneCountInString(rows[i].au))
		wDOI = max(wDOI, len(r.DOI))
	}
	// columns: id src year author flag title doi; the title takes what is left
	wTitle := width() - (wID + 3 + 4 + wAu + 6 + wDOI + 6*2)
	wTitle = max(wTitle, 20)
	line := func(r row) {
		fmt.Printf("%s  %-3s  %-4s  %s  %-6s  %s  %s\n", pad(r.id, wID), r.src, r.year, pad(r.au, wAu), r.flag,
			pad(trunc(r.title, wTitle), wTitle), r.doi)
	}
	line(row{"ID", "SRC", "YEAR", "AUTHOR", "TITLE", "DOI", "FLAGS"})
	for _, r := range rows {
		line(r)
	}
}
