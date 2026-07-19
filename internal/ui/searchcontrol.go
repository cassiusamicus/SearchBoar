package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
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
		a.setStatus("No search location selected (see Workspace Builder tab)")
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
	a.start.view.clear()
	a.tabs.SelectIndex(tabIndexResults)

	locOpts := a.locations.locationOptions()
	base := a.searchOptionsTemplate()

	// Show whatever's already indexed for this exact content pattern (via
	// Common Search Terms) immediately, before the real search does
	// anything -- ripgrep or otherwise. It's necessarily stale (indexed at
	// some earlier point, maybe under different locations) so it's cleared
	// the moment the live search actually produces its own first result or
	// progress update (see clearCacheSeedOnce below); this is a preview to
	// look at while the real search runs, not a replacement for it.
	if n := a.seedFromCache(base); n > 0 {
		a.setStatus(fmt.Sprintf("Showing %d cached result(s) for %q while searching...", n, strings.TrimSpace(base.ContentPattern)))
	} else {
		a.setStatus("Searching...")
	}

	a.searchButton.Disable()
	a.stopButton.Enable()
	a.results.stopBtn.Enable()
	a.progressBar.Show()
	a.progressBar.SetValue(0)
	a.bottomStopBtn.Show()

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelSearch = cancel

	go a.runUnifiedSearch(ctx, base, locOpts)
}

// seedFromCache looks up opts.ContentPattern as an exact, already-indexed
// Common Search Term (see internal/cache's term_matches table) and, if
// found, immediately populates the results views with it -- filtered by
// opts.FilePattern so a narrower current search doesn't show entries the
// index run (which always covers every file) wouldn't have matched. Exclude
// globs/size filters aren't re-applied here, unlike the real engine: this
// is a best-effort instant preview, not required to be exactly what the
// live search will find, and it's replaced by real results within moments
// regardless (see clearCacheSeedOnce). Returns the number of results shown.
func (a *App) seedFromCache(opts search.Options) int {
	term := strings.TrimSpace(opts.ContentPattern)
	if !opts.ContentEnabled || term == "" {
		return 0
	}
	matches, err := a.cache.GetTermMatches(term)
	if err != nil || len(matches) == 0 {
		return 0
	}
	filenameRe, err := search.CompileRegex(opts.FilePattern, opts.CaseSensitive)
	if err != nil {
		return 0
	}
	results := groupTermMatches(matches, filenameRe)
	for _, r := range results {
		a.searchResults = append(a.searchResults, r)
		a.results.addResult(r)
		a.start.view.addResult(r)
	}
	a.results.resort()
	a.start.view.resort()
	return len(results)
}

// groupTermMatches turns the flattened (one row per match) storage shape
// cache.GetTermMatches returns back into one model.FileResult per file,
// keeping only files whose name matches filenameRe. Relies on
// GetTermMatches' own "ORDER BY path, line_num" to make grouping a single
// pass instead of needing a map.
func groupTermMatches(matches []cache.TermMatch, filenameRe *regexp.Regexp) []model.FileResult {
	var out []model.FileResult
	for _, m := range matches {
		name := filepath.Base(m.Path)
		cm := model.ContentMatch{LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines}
		if len(out) > 0 && out[len(out)-1].Path == m.Path {
			out[len(out)-1].Matches = append(out[len(out)-1].Matches, cm)
			continue
		}
		if filenameRe != nil && !filenameRe.MatchString(name) {
			continue
		}
		out = append(out, model.FileResult{
			FileEntry: model.FileEntry{
				Path: m.Path, Name: name, ModTime: m.ModTime, Size: m.Size, DisplayPath: m.DisplayPath,
			},
			Matches: []model.ContentMatch{cm},
		})
	}
	return out
}

// restoreLastSearch pre-fills the Search Builder's filename pattern,
// pre-checks the Workspace Builder tree, and repopulates the Results tab
// with whatever the most recently completed search found, all loaded from
// config -- so "View All Results" from the Start tab's Recent Results
// isn't empty on a fresh launch just because no search has run yet this
// session. RecentResults persists match content too (see
// config.RecentResult), so a restored result shows the same highlighted
// context a fresh search would, not just its path/size/date.
func (a *App) restoreLastSearch() {
	if a.cfg.Recent.LastFilePattern != "" {
		a.builder.fileEntry.SetText(a.cfg.Recent.LastFilePattern)
	}
	if len(a.cfg.Recent.ContentPatterns) > 0 {
		a.builder.contentCombo.SetText(a.cfg.Recent.ContentPatterns[0])
	}
	a.locations.restoreCheckedPaths(a.cfg.Recent.Paths)

	if len(a.cfg.RecentResults) > 0 {
		a.searchResults = make([]model.FileResult, len(a.cfg.RecentResults))
		for i, r := range a.cfg.RecentResults {
			matches := make([]model.ContentMatch, len(r.Matches))
			for j, m := range r.Matches {
				matches[j] = model.ContentMatch{LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines}
			}
			a.searchResults[i] = model.FileResult{
				FileEntry: model.FileEntry{
					Path:        r.Path,
					Name:        filepath.Base(r.Path),
					ModTime:     parseModTime(r.Modified),
					Size:        r.SizeBytes,
					DisplayPath: r.DisplayPath,
				},
				Matches: matches,
			}
		}
		a.results.resort()
		a.start.view.resort()
	}

	// The Start tab's own build() already ran a refresh() before this
	// method had a chance to apply any of the above, so its quick
	// fields/location summary would otherwise show stale pre-restore
	// defaults until the user did something to trigger a redraw.
	a.start.refresh()
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

	// The cache-seeded preview (see seedFromCache) is stale by definition --
	// swap it out for the real thing the moment the live search actually
	// produces its own first result, from whichever root gets there first,
	// rather than leaving it up for the whole search or trying to merge the
	// two (which risks duplicate entries for a file both find). Also passed
	// to finishSearch so a live search that legitimately finds nothing
	// still clears a stale preview instead of leaving it showing forever.
	// sync.OnceFunc so it only fires once no matter which caller reaches it
	// first.
	clearCacheSeedOnce := sync.OnceFunc(func() {
		a.searchResults = nil
		a.results.clear()
		a.start.view.clear()
	})

	roots, err := a.netEng.ResolveRoots(ctx, locOpts, logf)
	if err != nil && ctx.Err() != nil {
		a.finishSearch(ctx.Err(), base.FilePattern, locOpts.LocalRoots, clearCacheSeedOnce)
		return
	}
	if len(roots) == 0 {
		a.setStatus("No search locations found")
		a.finishSearch(nil, base.FilePattern, locOpts.LocalRoots, clearCacheSeedOnce)
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

		if err := a.runOneRoot(ctx, opts, root, i+1, len(roots), clearCacheSeedOnce); err != nil {
			if err == context.Canceled || ctx.Err() != nil {
				searchErr = context.Canceled
				break
			}
			searchErr = err
		}
	}

	a.finishSearch(searchErr, base.FilePattern, locOpts.LocalRoots, clearCacheSeedOnce)
}

func (a *App) runOneRoot(ctx context.Context, opts search.Options, root netsearch.ResolvedRoot, rootIndex, rootTotal int, clearCacheSeedOnce func()) error {
	results := make(chan model.FileResult)
	progress := make(chan search.Progress)
	done := make(chan error, 1)

	go func() { done <- a.searchEng.Run(ctx, opts, results, progress) }()

	// runOnUI (fyne.Do) queues onto Fyne's own unbounded, async work queue --
	// it doesn't block here, but it also doesn't get un-queued by a later
	// cancellation. A fast local search over many files can produce results
	// far quicker than the UI thread can turn each one into a result card
	// (see resultsview.go's addResult, which re-lays-out the whole card
	// list on every call), so by the time Stop is clicked, a large backlog
	// of already-queued card-building work can still be sitting in that
	// queue -- and since nothing before this fix ever checked cancellation
	// here, every remaining result kept adding to it regardless, making a
	// cancel look like it had no effect while the UI visibly kept crawling
	// through results for a long time afterward. Checking ctx.Err() before
	// queuing each one stops the backlog from growing further the moment
	// Stop is clicked, instead of only stopping new candidates being
	// searched (which was already happening, just not enough on its own).
	resultsOpen, progressOpen := true, true
	for resultsOpen || progressOpen {
		select {
		case r, ok := <-results:
			if !ok {
				resultsOpen = false
				continue
			}
			if ctx.Err() != nil {
				continue
			}
			if root.DisplayPrefix != "" {
				if rel, err := filepath.Rel(root.Path, r.Path); err == nil {
					r.DisplayPath = root.DisplayPrefix + "/" + filepath.ToSlash(rel)
				}
			}
			runOnUI(func() {
				// Re-checked here, not just before queuing above: Fyne's
				// work queue is unbounded, and a fast search (real
				// documents with several matches each, so each card is
				// non-trivial to build, are plenty fast enough to trigger
				// this) can get results queued well ahead of what the UI
				// thread has actually gotten around to rendering. Without
				// this, everything already sitting in the queue at the
				// moment Stop is clicked still pays its full build cost
				// before the backlog visibly stops growing -- checking
				// again here means each already-queued closure becomes a
				// near-no-op instead, so the backlog drains almost
				// immediately rather than only stopping new results from
				// being added to it.
				if ctx.Err() != nil {
					return
				}
				clearCacheSeedOnce()
				a.searchResults = append(a.searchResults, r)
				a.results.addResult(r)
				a.start.view.addResult(r)
			})
		case p, ok := <-progress:
			if !ok {
				progressOpen = false
				continue
			}
			if ctx.Err() != nil {
				continue
			}
			runOnUI(func() {
				if ctx.Err() != nil {
					return
				}
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
func (a *App) finishSearch(err error, filePattern string, searchPaths []string, clearCacheSeedOnce func()) {
	runOnUI(func() {
		a.searchButton.Enable()
		a.stopButton.Disable()
		a.results.stopBtn.Disable()
		a.progressBar.Hide()
		a.bottomStopBtn.Hide()
		a.cancelSearch = nil
		if err == nil {
			// A search that ran to completion and found nothing proves the
			// cache-seeded preview (if clearCacheSeedOnce hasn't already
			// fired for a real result) is stale, so clear it rather than
			// leaving it showing forever. A stopped or errored search never
			// got to prove that either way, so it's left alone -- whatever
			// was on screen (cached preview or partial live results) stays.
			clearCacheSeedOnce()
		}
		// Results stream in during the search in arrival order (see
		// addResult) rather than being fully re-sorted after every single
		// one -- rebuilding every result card on every arrival would get
		// slower as the result set grows. The real sort order (default:
		// Number of Hits) is applied once here, whether the search
		// finished, errored, or was stopped early with partial results.
		a.results.resort()
		a.start.view.resort()
		if err != nil && err != context.Canceled {
			a.statusBar.SetText("Search error: " + err.Error())
			return
		}
		a.statusBar.SetText(searchCompleteText(len(a.searchResults), err == context.Canceled))

		recent := make([]recentResultSource, len(a.searchResults))
		for i, r := range a.searchResults {
			matches := make([]recentMatchSource, len(r.Matches))
			for j, m := range r.Matches {
				matches[j] = recentMatchSource{LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines}
			}
			recent[i] = recentResultSource{Path: r.Path, DisplayPath: r.DisplayPath, Modified: formatModTime(r.ModTime), Size: r.Size, Matches: matches}
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
