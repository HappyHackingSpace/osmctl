// Command gendocs writes the command reference (docs/commands) and man pages
// (docs/man) from the cobra command tree, so they never drift from the code.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/happyhackingspace/osmctl/internal/cli"
)

func main() {
	root := cli.NewRoot("")
	root.DisableAutoGenTag = true

	md := filepath.Join("docs", "commands")
	if err := os.RemoveAll(md); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(md, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := doc.GenMarkdownTree(root, md); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "-man" {
		man := filepath.Join("docs", "man")
		if err := os.MkdirAll(man, 0o755); err != nil {
			log.Fatal(err)
		}
		if err := doc.GenManTree(root, &doc.GenManHeader{Title: "OSMCTL", Section: "1"}, man); err != nil {
			log.Fatal(err)
		}
	}
}
