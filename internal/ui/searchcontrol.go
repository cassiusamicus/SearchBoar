package ui

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"

	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
	"codeberg.org/cassiusamicus/Utilities/internal/search"
)

// currentContentRegex compiles the Search Builder tab's current content
// pattern the same way the engine does, so match highlighting in the
// Results tab is exactly what the engine actually matched against.
func (a *App) currentContentRegex() *regexp.Regexp {
	if !a.builder.contentEnabled.Checked || a.builder.contentCombo.Text == "" {
		return nil
	}
	re, err := search.CompileRegex(a.builder.contentCombo.Text, a.builder.caseCheck.Checked)
	if err != nil {
		return nil
	}
	return re
}

// searchOptionsTemplate builds the pattern/filter half of search.Options
// from the Search Builder tab; Dir and ExcludeDirs are filled in per
// resolved root.
func (a *App) searchOptionsTemplate() search.Options {
	return search.Options{
		FilePattern:    a.builder.fileEntry.Text,
		ContentPattern: a.builder.contentCombo.Text,
		ContentEnabled: a.builder.contentEnabled.Checked,
		Recursive:      a.builder.recursiveCheck.Checked,
		CaseSensitive:  a.builder.caseCheck.Checked,
		IncludeHidden:  a.builder.hiddenCheck.Checked,
		ContextBefore:  a.builder.beforeSpin.value,
		ContextAfter:   a.builder.afterSpin.value,
		MinSizeBytes:   a.builder.minSizeBytes(),
		MaxSizeBytes:   a.builder.maxSizeBytes(),
		ExcludeGlobs:   search.SplitExcludePatterns(a.builder.excludeEntry.Text),
	}
}

func (a *App) startSearch() {
	if !a.locations.anyLocationSelected() {
		a.setStatus("No search location selected (see Search Locations tab)")
		return
	}
	if a.cancelSearch != nil {
		a.cancelSearch() // a search is already running; cancel it before starting a new one
	}
	a.netEng.Mounts.UnmountAll(context.Background()) // drop any mounts from a previous search before starting a new one

	if a.builder.contentEnabled.Checked {
		a.cfg.Recent.AddContentPattern(a.builder.contentCombo.Text)
		a.builder.contentCombo.SetOptions(a.cfg.Recent.ContentPatterns)
	}

	a.searchResults = nil
	a.results.clear()
	a.tabs.SelectIndex(tabIndexResults)

	a.searchButton.Disable()
	a.stopButton.Enable()
	a.progressBar.Show()
	a.progressBar.SetValue(0)
	a.setStatus("Searching...")

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSearch = cancel

	locOpts := a.locations.locationOptions()
	base := a.searchOptionsTemplate()

	go a.runUnifiedSearch(ctx, base, locOpts)
}

// restoreLastSearch pre-fills the Search Builder's filename pattern and
// pre-checks the Search Locations tree with whatever the most recently
// completed search used, loaded from config -- so the Start tab isn't the
// only place that remembers.
func (a *App) restoreLastSearch() {
	if a.cfg.Recent.LastFilePattern != "" {
		a.builder.fileEntry.SetText(a.cfg.Recent.LastFilePattern)
	}
	a.locations.restoreCheckedPaths(a.cfg.Recent.Paths)
}

func (a *App) stopSearch() {
	if a.cancelSearch != nil {
		a.cancelSearch()
	}
}

// runUnifiedSearch resolves every requested location (local drives, SMB
// shares, NFS exports -- mounting the latter two as needed) and then
// searches each one with the same regex-based engine, so local and
// network search share one pattern language and one feature set.
func (a *App) runUnifiedSearch(ctx context.Context, base search.Options, locOpts netsearch.LocationOptions) {
	logf := func(level, msg string) {
		a.setStatus("[" + level + "] " + msg)
	}

	roots, err := a.netEng.ResolveRoots(ctx, locOpts, logf)
	if err != nil && ctx.Err() != nil {
		a.finishSearch(ctx.Err(), base.FilePattern, locOpts.LocalRoots)
		return
	}
	if len(roots) == 0 {
		a.setStatus("No search locations found")
		a.finishSearch(nil, base.FilePattern, locOpts.LocalRoots)
		return
	}

	var searchErr error
	for i, root := range roots {
		if ctx.Err() != nil {
			searchErr = ctx.Err()
			break
		}

		opts := base
		opts.Dir = root.Path
		if root.DisplayPrefix == "" {
			opts.ExcludeDirs = a.locations.excludeDirsFor(root.Path)
		}

		if err := a.runOneRoot(ctx, opts, root, i+1, len(roots)); err != nil {
			if err == context.Canceled || ctx.Err() != nil {
				searchErr = context.Canceled
				break
			}
			searchErr = err
		}
	}

	a.finishSearch(searchErr, base.FilePattern, locOpts.LocalRoots)
}

func (a *App) runOneRoot(ctx context.Context, opts search.Options, root netsearch.ResolvedRoot, rootIndex, rootTotal int) error {
	results := make(chan model.FileResult)
	progress := make(chan search.Progress)
	done := make(chan error, 1)

	go func() { done <- a.searchEng.Run(ctx, opts, results, progress) }()

	resultsOpen, progressOpen := true, true
	for resultsOpen || progressOpen {
		select {
		case r, ok := <-results:
			if !ok {
				resultsOpen = false
				continue
			}
			if root.DisplayPrefix != "" {
				if rel, err := filepath.Rel(root.Path, r.Path); err == nil {
					r.DisplayPath = root.DisplayPrefix + "/" + filepath.ToSlash(rel)
				}
			}
			runOnUI(func() {
				a.searchResults = append(a.searchResults, r)
				a.results.addResult(r)
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
				a.statusBar.SetText(searchStatusText(p, rootIndex, rootTotal))
			})
		}
	}

	return <-done
}

// finishSearch runs entirely on the UI goroutine (via runOnUI) so that
// reading a.searchResults here -- to update the status bar and record the
// Start tab's history -- never races with the same-thread appends made
// while results were streaming in.
func (a *App) finishSearch(err error, filePattern string, searchPaths []string) {
	runOnUI(func() {
		a.searchButton.Enable()
		a.stopButton.Disable()
		a.progressBar.Hide()
		a.cancelSearch = nil
		if err != nil && err != context.Canceled {
			a.statusBar.SetText("Search error: " + err.Error())
			return
		}
		a.statusBar.SetText(searchCompleteText(len(a.searchResults), err == context.Canceled))

		recent := make([]recentResultSource, len(a.searchResults))
		for i, r := range a.searchResults {
			recent[i] = recentResultSource{Path: r.Path, DisplayPath: r.DisplayPath, Modified: formatModTime(r.ModTime), Size: r.Size}
		}
		a.start.recordSearch(filePattern, searchPaths, recent)
	})
}

func searchStatusText(p search.Progress, rootIndex, rootTotal int) string {
	location := ""
	if rootTotal > 1 {
		location = " (location " + strconv.Itoa(rootIndex) + "/" + strconv.Itoa(rootTotal) + ")"
	}
	if p.Total > 0 {
		return "Searching" + location + "... " + strconv.Itoa(p.Searched) + "/" + strconv.Itoa(p.Total) + " (" + strconv.Itoa(p.Found) + " found)"
	}
	return "Searching" + location + "... " + strconv.Itoa(p.Found) + " found"
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
