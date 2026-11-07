package cmd

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// config is read from $XDG_CONFIG_HOME/gorxiv/config (~/.config/gorxiv/config):
//
//	# comments and blank lines are ignored
//	pdf_dir = ~/papers
//	viewer  = zathura --fork
type config struct {
	PDFDir string
	Viewer string
}

func configDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "gorxiv")
}

var cfg = loadConfig()

func loadConfig() config {
	c := config{PDFDir: filepath.Join(configDir(), "pdfs"), Viewer: "zathura"}
	f, err := os.Open(filepath.Join(configDir(), "config"))
	if err != nil {
		return c // ponytail: missing or unreadable config = defaults
	}
	defer f.Close()
	return parseConfig(f, c)
}

func parseConfig(r io.Reader, c config) config {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || strings.HasPrefix(k, "#") || v == "" {
			continue
		}
		switch k {
		case "pdf_dir":
			c.PDFDir = expandHome(v)
		case "viewer":
			c.Viewer = v
		}
	}
	return c
}

func expandHome(p string) string {
	if h, err := os.UserHomeDir(); err == nil && (p == "~" || strings.HasPrefix(p, "~/")) {
		return filepath.Join(h, p[1:])
	}
	return p
}
