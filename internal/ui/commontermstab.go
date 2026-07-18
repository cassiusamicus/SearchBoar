package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
	"codeberg.org/cassiusamicus/Utilities/internal/search"
)

// commonTermsTab is the "Common Search Terms" tab: a small, user-curated
// list of content terms searched often enough to be worth keeping an
// up-to-date index of. Adding a term immediately searches for it (over the
// current Search Locations selection) and saves the matches to the cache
// database; Reindex repeats that for a term whose files may have changed
// since. This is deliberately on-demand, not a background filesystem
// indexer -- nothing is searched until the user asks for it.
type commonTermsTab struct {
	app *App

	entry *widget.Entry
	list  *widget.List

	terms    []cache.TermInfo
	indexing map[string]bool
}

func newCommonTermsTab(a *App) *commonTermsTab {
	return &commonTermsTab{app: a, indexing: map[string]bool{}}
}

func (t *commonTermsTab) build() fyne.CanvasObject {
	t.entry = widget.NewEntry()
	t.entry.SetPlaceHolder("Add a term you search for often...")
	t.entry.OnSubmitted = func(string) { t.addTerm() }
	addBtn := widget.NewButtonWithIcon("Add Term", theme.ContentAddIcon(), func() { t.addTerm() })
	addBtn.Importance = widget.HighImportance

	t.list = widget.NewList(
		func() int { return len(t.terms) },
		func() fyne.CanvasObject { return newTermRow() },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateRow(id, o.(*termRow)) },
	)

	header := widget.NewLabel("Terms you search for often. Adding one searches your current Search Locations right away and saves the matches here; use the refresh button on any term to update it later (e.g. after files change).")
	header.Wrapping = fyne.TextWrapWord

	top := container.NewVBox(header, container.NewBorder(nil, nil, nil, addBtn, t.entry))

	t.refresh()
	return container.NewBorder(top, nil, nil, nil, t.list)
}

func (t *commonTermsTab) addTerm() {
	term := strings.TrimSpace(t.entry.Text)
	if term == "" {
		return
	}
	t.entry.SetText("")
	if err := t.app.cache.AddTerm(term); err != nil {
		t.app.setStatus("Failed to add term: " + err.Error())
		return
	}
	t.refresh()
	t.app.reindexTerm(term)
}

func (t *commonTermsTab) removeTerm(term string) {
	if err := t.app.cache.RemoveTerm(term); err != nil {
		t.app.setStatus("Failed to remove term: " + err.Error())
		return
	}
	t.refresh()
}

// refresh reloads the term list from the cache database -- called on build,
// whenever the tab becomes visible, and after every add/remove/reindex.
func (t *commonTermsTab) refresh() {
	if t.list == nil {
		return
	}
	terms, err := t.app.cache.ListTerms()
	if err != nil {
		t.app.setStatus("Failed to load common search terms: " + err.Error())
		return
	}
	t.terms = terms
	t.list.Refresh()
}

// setIndexing flips term's in-progress flag and refreshes the list so its
// row's status text updates immediately.
func (t *commonTermsTab) setIndexing(term string, on bool) {
	if on {
		t.indexing[term] = true
	} else {
		delete(t.indexing, term)
	}
	if t.list != nil {
		t.list.Refresh()
	}
}

func (t *commonTermsTab) updateRow(id widget.ListItemID, row *termRow) {
	if id >= len(t.terms) {
		row.termLabel.SetText("")
		row.statusLabel.SetText("")
		return
	}
	info := t.terms[id]
	term := info.Term
	row.termLabel.SetText(term)

	switch {
	case t.indexing[term]:
		row.statusLabel.SetText("Indexing...")
	case info.LastIndexedAt.IsZero():
		row.statusLabel.SetText("Not indexed yet")
	default:
		row.statusLabel.SetText(fmt.Sprintf("%d match(es) • indexed %s", info.MatchCount, formatModTime(info.LastIndexedAt)))
	}

	row.reindexBtn.OnTapped = func() { t.app.reindexTerm(term) }
	row.removeBtn.OnTapped = func() { t.removeTerm(term) }
}

// termRow is one row's widgets, reused by widget.List across rows. It wraps
// its content in widget.BaseWidget/NewSimpleRenderer (the same pattern
// tappableBox uses) rather than returning a bare *fyne.Container -- a raw
// Container satisfies fyne.CanvasObject but isn't a fyne.Widget, and
// widget.List's item lifecycle (theme-scope binding, refresh) silently
// failed to render rows at all when this returned a bare Container instead
// of a proper Widget.
type termRow struct {
	widget.BaseWidget

	box         *fyne.Container
	termLabel   *widget.Label
	statusLabel *widget.Label
	reindexBtn  *widget.Button
	removeBtn   *widget.Button
}

func newTermRow() *termRow {
	termLabel := widget.NewLabel("")
	statusLabel := widget.NewLabel("")
	statusLabel.Importance = widget.LowImportance
	reindexBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), nil)
	reindexBtn.Importance = widget.LowImportance
	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	removeBtn.Importance = widget.LowImportance

	box := container.NewBorder(nil, nil, nil, container.NewHBox(statusLabel, reindexBtn, removeBtn), termLabel)
	row := &termRow{box: box, termLabel: termLabel, statusLabel: statusLabel, reindexBtn: reindexBtn, removeBtn: removeBtn}
	row.ExtendBaseWidget(row)
	return row
}

func (r *termRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.box)
}

// reindexTerm runs a fresh content search for term over the current Search
// Locations selection and replaces its stored matches. DisableRipgrep is
// set so this always goes through the engine's cache-populating walk path
// (see search.Options.DisableRipgrep) -- ripgrep would be faster for this
// one run, but it reads files itself and bypasses Engine.Cache entirely, so
// using it here would defeat the point of building an index.
func (a *App) reindexTerm(term string) {
	if !a.locations.anyLocationSelected() {
		a.setStatus("No search location selected -- can't index \"" + term + "\" (see Search Locations tab)")
		return
	}
	locOpts := a.locations.locationOptions()
	base := search.Options{
		FilePattern:    ".*",
		ContentPattern: term,
		ContentEnabled: true,
		Recursive:      true,
		DisableRipgrep: true,
	}

	a.commonTerms.setIndexing(term, true)
	a.setStatus("Indexing \"" + term + "\"...")

	go func() {
		ctx := context.Background()
		var matches []cache.TermMatch

		roots, err := a.netEng.ResolveRoots(ctx, locOpts, func(level, msg string) {})
		if err == nil {
			for _, root := range roots {
				opts := base
				opts.Dir = root.Path
				if root.DisplayPrefix == "" {
					opts.ExcludeDirs = a.locations.excludeDirsFor(root.Path)
				}
				matches = append(matches, collectTermMatches(ctx, a.searchEng, opts, root)...)
			}
		}

		if serr := a.cache.SaveTermMatches(term, matches); serr != nil {
			runOnUI(func() { a.setStatus("Failed to save index for \"" + term + "\": " + serr.Error()) })
			return
		}

		runOnUI(func() {
			a.commonTerms.setIndexing(term, false)
			a.commonTerms.refresh()
			a.setStatus(fmt.Sprintf("Indexed \"%s\": %d match(es)", term, len(matches)))
		})
	}()
}

// collectTermMatches runs one root's search to completion (blocking) and
// flattens its results into TermMatch rows, applying the same
// DisplayPath-from-mount-prefix logic runOneRoot uses for a normal search
// (see searchcontrol.go) so network-share matches show their network-style
// path rather than a local mount-point path.
func collectTermMatches(ctx context.Context, eng *search.Engine, opts search.Options, root netsearch.ResolvedRoot) []cache.TermMatch {
	results := make(chan model.FileResult)
	done := make(chan error, 1)
	go func() { done <- eng.Run(ctx, opts, results, nil) }()

	var out []cache.TermMatch
	for r := range results {
		displayPath := r.DisplayPath
		if root.DisplayPrefix != "" {
			if rel, err := filepath.Rel(root.Path, r.Path); err == nil {
				displayPath = root.DisplayPrefix + "/" + filepath.ToSlash(rel)
			}
		}
		for _, m := range r.Matches {
			out = append(out, cache.TermMatch{
				Path: r.Path, DisplayPath: displayPath, ModTime: r.ModTime, Size: r.Size,
				LineNum: m.LineNum, ContextStartLine: m.ContextStartLine, ContextLines: m.ContextLines,
			})
		}
	}
	<-done
	return out
}
