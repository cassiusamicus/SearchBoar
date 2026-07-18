package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/favorites"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
	"codeberg.org/cassiusamicus/Utilities/internal/search"
	"codeberg.org/cassiusamicus/Utilities/internal/storedsearches"
)

// App is the shared hub every tab/dialog closes over: config, engines,
// stores, and the handful of top-level widgets other tabs need to update
// (status bar, tab switcher, search/stop buttons).
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	cfg      *config.Config
	engine   *search.Engine
	favStore *favorites.Store
	ssStore  *storedsearches.Store

	tabs        *container.AppTabs
	statusBar   *widget.Label
	progressBar *widget.ProgressBar

	searchButton *widget.ToolbarAction
	stopButton   *widget.ToolbarAction

	cancelSearch context.CancelFunc

	// results holds the full result set of the most recently completed (or
	// in-progress) search, shared by the Details/Overview tabs and the
	// favorite/context-menu actions.
	results []model.FileResult

	basic    *basicTab
	advanced *advancedTab
	details  *detailsTab
	overview *overviewTab
	favTab   *favoritesTab
}

// runOnUI marshals fn onto the Fyne UI goroutine. Every mutation of a
// widget from a background goroutine (search progress, results, network
// search log lines) must go through this.
func runOnUI(fn func()) {
	fyne.Do(fn)
}

func (a *App) setStatus(msg string) {
	runOnUI(func() { a.statusBar.SetText(msg) })
}
