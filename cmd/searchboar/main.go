// Command searchboar is a combined local and network file search tool,
// merging the features of the original SearchBoar and LanSearch Python/GTK3
// applications into a single Go/Fyne binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/cassiusamicus/SearchBoar/internal/model"
	"github.com/cassiusamicus/SearchBoar/internal/search"
	"github.com/cassiusamicus/SearchBoar/internal/ui"
)

func main() {
	cli := flag.Bool("cli", false, "run one search from the terminal instead of launching the GUI")
	dir := flag.String("dir", ".", "directory to search")
	filePattern := flag.String("file-pattern", ".*", "filename regex")
	content := flag.String("content", "", "content regex (empty = filename-only search)")
	recursive := flag.Bool("recursive", true, "search subdirectories")
	caseSensitive := flag.Bool("case-sensitive", false, "case-sensitive matching")
	hidden := flag.Bool("hidden", false, "include hidden files")
	before := flag.Int("before", 2, "context lines before a match")
	after := flag.Int("after", 2, "context lines after a match")
	flag.Parse()

	if !*cli {
		ui.Run()
		return
	}

	opts := search.Options{
		Dir:            *dir,
		FilePattern:    *filePattern,
		ContentPattern: *content,
		ContentEnabled: *content != "",
		Recursive:      *recursive,
		CaseSensitive:  *caseSensitive,
		IncludeHidden:  *hidden,
		ContextBefore:  *before,
		ContextAfter:   *after,
	}

	e := search.NewEngine()
	results := make(chan model.FileResult)
	progress := make(chan search.Progress)
	done := make(chan error, 1)

	go func() {
		done <- e.Run(context.Background(), opts, results, progress)
	}()

	resultsOpen, progressOpen := true, true
	found := 0
	for resultsOpen || progressOpen {
		select {
		case r, ok := <-results:
			if !ok {
				resultsOpen = false
				continue
			}
			found++
			fmt.Printf("%s (%d bytes)\n", r.Path, r.Size)
			for _, m := range r.Matches {
				fmt.Printf("  line %d:\n", m.LineNum)
				for i, line := range m.ContextLines {
					fmt.Printf("    %4d: %s\n", m.ContextStartLine+i, line)
				}
			}
		case p, ok := <-progress:
			if !ok {
				progressOpen = false
				continue
			}
			fmt.Fprintf(os.Stderr, "\rsearched %d/%d, found %d", p.Searched, p.Total, p.Found)
		}
	}
	fmt.Fprintln(os.Stderr)

	if err := <-done; err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("%d file(s) matched\n", found)
}
