package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	h, _ := os.UserHomeDir()
	c := parseConfig(strings.NewReader("# x = y\n\npdf_dir = ~/papers\nviewer=zathura --fork\nbogus\nviewer =\n"),
		config{PDFDir: "def", Viewer: "def"})
	if c.PDFDir != filepath.Join(h, "papers") || c.Viewer != "zathura --fork" {
		t.Fatalf("%+v", c)
	}
}
