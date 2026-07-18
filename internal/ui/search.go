package ui

import (
	"context"
	"regexp"
	"strconv"

	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
	"codeberg.org/cassiusamicus/Utilities/internal/search"
)

// currentContentRegex compiles the Basic tab's current content pattern the
// same way the engine does, so match highlighting in the Details/Overview
// tabs is exactly what the engine actually matched against.
func (a *App) currentContentRegex() *regexp.Regexp {
	if !a.basic.contentEnabled.Checked || a.basic.contentCombo.Text == "" {
		return nil
	}
	re, err := search.CompileRegex(a.basic.contentCombo.Text, a.caseSensitive())
	if err != nil {
		return nil
	}
	return re
}

func (a *App) caseSensitive() bool { return a.basic.caseCheck.Checked }

func (a *App) buildSearchOptions() search.Options {
	return search.Options{
		Dir:            a.basic.dirCombo.Text,
		FilePattern:    a.basic.fileEntry.Text,
		ContentPattern: a.basic.contentCombo.Text,
		ContentEnabled: a.basic.contentEnabled.Checked,
		Recursive:      a.basic.recursiveCheck.Checked,
		CaseSensitive:  a.basic.caseCheck.Checked,
		IncludeHidden:  a.basic.hiddenCheck.Checked,
		ContextBefore:  a.advanced.beforeSpin.value,
		ContextAfter:   a.advanced.afterSpin.value,
		MinSizeBytes:   a.advanced.minSizeBytes(),
		MaxSizeBytes:   a.advanced.maxSizeBytes(),
		ExcludeGlobs:   search.SplitExcludePatterns(a.advanced.excludeEntry.Text),
	}
}

func (a *App) startSearch() {
	if a.basic.dirCombo.Text == "" {
		a.setStatus("No search directory specified")
		return
	}
	if a.cancelSearch != nil {
		a.cancelSearch() // a search is already running; cancel it before starting a new one
	}

	opts := a.buildSearchOptions()

	a.cfg.Recent.AddPath(opts.Dir)
	a.basic.dirCombo.SetOptions(a.cfg.Recent.Paths)
	if opts.ContentEnabled {
		a.cfg.Recent.AddContentPattern(opts.ContentPattern)
		a.basic.contentCombo.SetOptions(a.cfg.Recent.ContentPatterns)
	}

	a.results = nil
	a.details.clear()
	a.overview.clear()
	a.tabs.SelectIndex(tabIndexDetails)

	a.searchButton.Disable()
	a.stopButton.Enable()
	a.progressBar.Show()
	a.progressBar.SetValue(0)
	a.setStatus("Searching...")

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSearch = cancel

	results := make(chan model.FileResult)
	progress := make(chan search.Progress)
	done := make(chan error, 1)

	go func() { done <- a.engine.Run(ctx, opts, results, progress) }()
	go a.consumeSearch(results, progress, done)
}

func (a *App) stopSearch() {
	if a.cancelSearch != nil {
		a.cancelSearch()
	}
}

func (a *App) consumeSearch(results <-chan model.FileResult, progress <-chan search.Progress, done <-chan error) {
	resultsOpen, progressOpen := true, true
	for resultsOpen || progressOpen {
		select {
		case r, ok := <-results:
			if !ok {
				resultsOpen = false
				continue
			}
			runOnUI(func() {
				a.results = append(a.results, r)
				a.details.resort()
				a.overview.addResult(r)
			})
		case p, ok := <-progress:
			if !ok {
				progressOpen = false
				continue
			}
			runOnUI(func() {
				if p.Total > 0 {
					a.progressBar.SetValue(float64(p.Searched) / float64(p.Total))
				}
				a.statusBar.SetText(searchStatusText(p))
			})
		}
	}

	err := <-done
	runOnUI(func() {
		a.searchButton.Enable()
		a.stopButton.Disable()
		a.progressBar.Hide()
		a.cancelSearch = nil
		if err != nil && err != context.Canceled {
			a.statusBar.SetText("Search error: " + err.Error())
			return
		}
		a.statusBar.SetText(searchCompleteText(len(a.results), err == context.Canceled))
	})
}

func searchStatusText(p search.Progress) string {
	if p.Total > 0 {
		return "Searching... " + strconv.Itoa(p.Searched) + "/" + strconv.Itoa(p.Total) + " (" + strconv.Itoa(p.Found) + " found)"
	}
	return "Searching... " + strconv.Itoa(p.Found) + " found"
}

func searchCompleteText(found int, canceled bool) string {
	suffix := " file(s) found"
	if canceled {
		return "Search stopped: " + strconv.Itoa(found) + suffix
	}
	return "Search complete: " + strconv.Itoa(found) + suffix
}

// openResult opens a search result's file with the OS default handler.
func (a *App) openResult(path string) error {
	return fsutil.OpenPath(path)
}
