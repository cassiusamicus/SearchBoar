package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// startTab is the "Start" tab: what you searched for and where last time,
// plus your most recent results, all persisted across restarts, with a
// Clear button to reset it.
type startTab struct {
	app *App

	termLabel   *widget.Label
	pathsBox    *fyne.Container
	resultsList *widget.List
}

func newStartTab(a *App) *startTab {
	return &startTab{app: a}
}

func (t *startTab) build() fyne.CanvasObject {
	t.termLabel = widget.NewLabel("")

	t.pathsBox = container.NewVBox()

	t.resultsList = widget.NewList(
		func() int { return len(t.app.cfg.RecentResults) },
		func() fyne.CanvasObject { return newTappableBox(widget.NewIcon(theme.FileIcon()), widget.NewLabel("")) },
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateResultRow(id, o.(*tappableBox)) },
	)

	clearBtn := widget.NewButton("Clear", func() { t.clear() })

	t.refresh()

	return container.NewVBox(
		widget.NewCard("Last Search Term", "", t.termLabel),
		widget.NewCard("Last Search Paths", "", t.pathsBox),
		widget.NewCard("Recent Results", "", container.NewVScroll(t.resultsList)),
		clearBtn,
	)
}

func (t *startTab) updateResultRow(id widget.ListItemID, box *tappableBox) {
	if id >= len(t.app.cfg.RecentResults) {
		return
	}
	r := t.app.cfg.RecentResults[id]
	path := r.Path
	label := r.DisplayPath
	if label == "" {
		label = path
	}
	box.SetObjects([]fyne.CanvasObject{widget.NewIcon(theme.FileIcon()), widget.NewLabel(filepath.Base(path) + "  —  " + label)})
	box.OnDoubleTapped = func() {
		if err := t.app.openResult(path); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	}
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		rec := fileRecord{Path: path, Name: filepath.Base(path), ModifiedStr: r.Modified, Size: r.SizeBytes, SizeHuman: r.SizeHuman}
		menu := t.app.fileContextMenu(rec, nil)
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

// refresh redraws the tab from the current config state; called at
// startup and after every completed search.
func (t *startTab) refresh() {
	if t.termLabel == nil {
		return // build() hasn't run yet
	}

	pattern := t.app.cfg.Recent.LastFilePattern
	content := ""
	if len(t.app.cfg.Recent.ContentPatterns) > 0 {
		content = t.app.cfg.Recent.ContentPatterns[0]
	}
	switch {
	case pattern == "" && content == "":
		t.termLabel.SetText("(no search yet)")
	case content == "":
		t.termLabel.SetText("Files: " + pattern)
	default:
		t.termLabel.SetText("Files: " + pattern + "    Containing: " + content)
	}

	t.pathsBox.Objects = nil
	if len(t.app.cfg.Recent.Paths) == 0 {
		t.pathsBox.Add(widget.NewLabel("(none)"))
	}
	for _, p := range t.app.cfg.Recent.Paths {
		t.pathsBox.Add(widget.NewLabel(p))
	}
	t.pathsBox.Refresh()

	t.resultsList.Refresh()
}

func (t *startTab) clear() {
	t.app.cfg.Recent.LastFilePattern = ""
	t.app.cfg.Recent.ContentPatterns = nil
	t.app.cfg.Recent.Paths = nil
	t.app.cfg.RecentResults = nil
	t.app.cfg.Save()
	t.refresh()
}

// recordSearch persists what was just searched for/where, and the first
// MaxRecentResults results, so the Start tab reflects it (including after
// a restart). Must be called from the UI goroutine (it re-reads config
// state and refreshes widgets directly, no internal runOnUI hop).
func (t *startTab) recordSearch(filePattern string, searchPaths []string, results []recentResultSource) {
	t.app.cfg.Recent.LastFilePattern = filePattern
	t.app.cfg.Recent.Paths = searchPaths

	recent := make([]config.RecentResult, 0, len(results))
	for i, r := range results {
		if i >= config.MaxRecentResults {
			break
		}
		recent = append(recent, config.RecentResult{
			Path: r.Path, DisplayPath: r.DisplayPath, Modified: r.Modified, SizeBytes: r.Size, SizeHuman: formatSize(r.Size),
		})
	}
	t.app.cfg.RecentResults = recent
	t.app.cfg.Save()

	t.refresh()
}

// recentResultSource is the minimal shape searchcontrol.go needs to hand
// off to recordSearch without importing the ui package's model dependency
// twice over.
type recentResultSource struct {
	Path        string
	DisplayPath string
	Modified    string
	Size        int64
}
