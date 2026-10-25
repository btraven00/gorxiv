package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/btraven00/gorxiv/epmc"
)

var pdfOut string

var pdfCmd = &cobra.Command{
	Use:   "pdf <id>",
	Short: "Download the open-access PDF of a record",
	Long: `Download a record's PDF, trying only openly available sources, in order:
  1. Europe PMC render (europepmc.org/articles/PMCxxx?pdf=render)
  2. NCBI PMC Article Datasets on AWS (open-access / author-manuscript subsets)
  3. fullTextUrlList entries with documentStyle=pdf (not subscription-only)
  4. free/open DOI links: a direct PDF, or a bioRxiv/medRxiv landing page -> .full.pdf
The body must start with %PDF-. Saved as <id>.pdf (DOIs slugged).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		c := client()
		r, _, err := c.Lookup(ctx, args[0])
		if err != nil {
			return err
		}
		if err := os.MkdirAll(pdfOut, 0o755); err != nil {
			return err
		}
		_, id := epmc.DetectID(args[0])
		dst := filepath.Join(pdfOut, slug(id)+".pdf")
		s, tried, err := c.FetchPDF(ctx, r, dst)
		for _, a := range tried {
			if a.Err != "" {
				fmt.Fprintf(os.Stderr, "  tried %s: %s\n", a.URL, a.Err)
			}
		}
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%d bytes\tsha256:%s\n  from %s\n", s.Path, s.Size, s.SHA256, s.URL)
		return nil
	},
}

func init() {
	pdfCmd.Flags().StringVarP(&pdfOut, "out", "o", ".", "output directory")
	rootCmd.AddCommand(pdfCmd)
}

var slugRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func slug(id string) string { return slugRE.ReplaceAllString(id, "_") }
