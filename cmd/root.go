package cmd

import (
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/btraven00/gorxiv/epmc"
)

// version is set with -ldflags "-X github.com/btraven00/gorxiv/cmd.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "gorxiv",
	Short: "Search Europe PMC, inspect records and fetch open-access PDFs",
	Long: `gorxiv searches Europe PMC (articles and preprints) from the terminal,
prints a record's metadata, and downloads its PDF when one is openly available.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true, // main prints the error
}

func Execute() error { return rootCmd.Execute() }

func client() *epmc.Client {
	return epmc.New("gorxiv/" + version + " (+https://github.com/btraven00/gorxiv)")
}

// width is the terminal width of stdout, else $COLUMNS, else 120.
func width() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 40 {
		return w
	}
	if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w > 40 {
		return w
	}
	return 120
}

func trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// wrap breaks s into lines of at most w runes, each prefixed with indent.
func wrap(s string, w int, indent string) string {
	var b strings.Builder
	line := 0
	for _, word := range strings.Fields(s) {
		n := utf8.RuneCountInString(word)
		if line > 0 && line+1+n > w {
			b.WriteString("\n")
			line = 0
		}
		if line == 0 {
			b.WriteString(indent)
		} else {
			b.WriteString(" ")
			line++
		}
		b.WriteString(word)
		line += n
	}
	return b.String()
}

// pad right-pads s with spaces to w runes (fmt's %-*s counts bytes).
func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-utf8.RuneCountInString(s)))
}
