package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/btraven00/gorxiv/epmc"
)

var showJSON bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print a record's metadata (PMID, PMCxxx, PPRxxx or DOI)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, n, err := client().Lookup(context.Background(), args[0])
		if err != nil {
			return err
		}
		if n > 1 {
			fmt.Fprintf(os.Stderr, "note: %d records match %s; showing %s:%s\n", n, args[0], r.Source, r.ID)
		}
		if showJSON {
			fmt.Println(string(r.Raw))
			return nil
		}
		printRecord(r)
		return nil
	},
}

func init() {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "print the raw core record")
	rootCmd.AddCommand(showCmd)
}

func printRecord(r *epmc.Record) {
	w := min(width(), 100)
	field := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			fmt.Printf("%-12s %s\n", k, v)
		}
	}
	block := func(k, v string) {
		if v != "" {
			fmt.Printf("%s\n%s\n", k, wrap(v, w-2, "  "))
		}
	}
	fmt.Println(wrap(epmc.StripTags(r.Title), w, ""))
	fmt.Println()
	field("Source", r.Source+":"+r.ID)
	venue := r.JournalInfo.Journal.Title
	if venue != "" {
		venue += " " + r.JournalInfo.Volume
		if r.JournalInfo.Issue != "" {
			venue += "(" + r.JournalInfo.Issue + ")"
		}
		if r.PageInfo != "" {
			venue += ":" + r.PageInfo
		}
	} else {
		venue = r.BookOrReportDetails.Publisher
	}
	field("Venue", venue)
	field("Published", strings.TrimSpace(r.FirstPublicationDate+" (year "+r.PubYear+")"))
	field("DOI", r.DOI)
	field("PMID", r.PMID)
	field("PMCID", r.PMCID)
	field("License", r.License)
	field("Open access", r.IsOpenAccess+"  (in Europe PMC: "+r.InEPMC+", PDF: "+r.HasPDF+")")
	if r.CitedByCount > 0 {
		field("Cited by", fmt.Sprint(r.CitedByCount))
	}
	if r.VersionNumber > 0 || len(r.VersionList.Version) > 0 {
		var vs []string
		for _, v := range r.VersionList.Version {
			vs = append(vs, fmt.Sprintf("v%d %s %s", v.VersionNumber, v.ID, v.FirstPublishDate))
		}
		field("Version", fmt.Sprintf("v%d; all: %s", r.VersionNumber, strings.Join(vs, ", ")))
	}
	fmt.Println()

	var names []string
	var affs []string
	seen := map[string]bool{}
	for _, a := range r.AuthorList.Author {
		names = append(names, a.FullName)
		for _, x := range a.Aff.List {
			if !seen[x.Affiliation] {
				seen[x.Affiliation] = true
				affs = append(affs, x.Affiliation)
			}
		}
	}
	if len(names) == 0 && r.AuthorString != "" {
		names = []string{r.AuthorString}
	}
	block(fmt.Sprintf("Authors (%d)", len(r.AuthorList.Author)), strings.Join(names, ", "))
	if len(affs) > 0 {
		fmt.Printf("Affiliations (%d unique)\n", len(affs))
		const maxAff = 8 // ponytail: short list; --json has them all
		for i, a := range affs {
			if i == maxAff {
				fmt.Printf("  … %d more (--json for all)\n", len(affs)-maxAff)
				break
			}
			fmt.Printf("  - %s\n", trunc(a, w-4))
		}
	}
	block("Keywords", strings.Join(r.KeywordList.Keyword, "; "))
	fmt.Println()
	block("Abstract", epmc.StripTags(r.AbstractText))
	if ft := r.FullTextURLList.FullTextURL; len(ft) > 0 {
		fmt.Println()
		fmt.Println("Full text")
		for _, f := range ft {
			fmt.Printf("  %-24s %-5s %-10s %s\n", f.Availability, f.DocumentStyle, f.Site, f.URL)
		}
	}
}
