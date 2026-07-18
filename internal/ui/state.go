package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/favorites"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
	"codeberg.org/cassiusamicus/Utilities/internal/search"
	"codeberg.org/cassiusamicus/Utilities/internal/storedsearches"
)

// App is the shared hub every tab/dialog closes over: config, engines,
// stores, and the handful of top-level widgets other tabs need to update
// (status bar, tab switcher, search/stop buttons).
//
// Local and network search share one pattern engine (internal/search,
// regex-based): internal/netsearch only discovers and mounts locations
// (local drives, SMB shares, NFS exports); once a location is a real
// filesystem path, it's searched exactly like any local directory.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	cfg       *config.Config
	searchEng *search.Engine
	netEng    *netsearch.Engine
	favStore  *favorites.Store
	ssStore   *storedsearches.Store
	cache     *cache.Cache // also referenced (nil-safe) via searchEng.Cache; kept here too for the About dialog's cache stats/clear button

	tabs        *container.AppTabs
	statusBar   *widget.Label
	progressBar *widget.ProgressBar

	searchButton *widget.ToolbarAction
	stopButton   *widget.ToolbarAction

	cancelSearch context.CancelFunc

	// searchResults holds the full result set of the most recently
	// completed (or in-progress) search, shared by the Results tab and the
	// favorite/context-menu actions.
	searchResults []model.FileResult

	start       *startTab
	builder     *searchBuilderTab
	locations   *locationsTab
	results     *resultsTab
	favTab      *favoritesTab
	favSearches *favoriteSearchesTab
	commonTerms *commonTermsTab
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
