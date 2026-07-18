package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/version"
)

const (
	tabIndexStart = iota
	tabIndexBuilder
	tabIndexLocations
	tabIndexResults
	tabIndexFavorites
	tabIndexFavSearches
	tabIndexCommonTerms
)

func (a *App) buildToolbar() *widget.Toolbar {
	a.searchButton = widget.NewToolbarAction(theme.SearchIcon(), func() { a.startSearch() })
	a.stopButton = widget.NewToolbarAction(theme.MediaStopIcon(), func() { a.stopSearch() })
	a.stopButton.Disable()

	t := widget.NewToolbar(
		toolbarWidgetItem{obj: wordmark()},
		widget.NewToolbarAction(theme.HomeIcon(), func() { a.tabs.SelectIndex(tabIndexStart) }),
		a.searchButton,
		widget.NewToolbarAction(theme.DocumentIcon(), func() { a.tabs.SelectIndex(tabIndexFavSearches) }),
		widget.NewToolbarAction(theme.ListIcon(), func() { a.tabs.SelectIndex(tabIndexFavorites) }),
		a.stopButton,
		widget.NewToolbarAction(theme.SettingsIcon(), func() { a.openConfigFile() }),
		widget.NewToolbarAction(theme.HelpIcon(), func() { a.showAboutDialog() }),
	)
	return t
}

// showAboutDialog also surfaces the search-result text cache's current size
// and a way to clear it -- the cache has no automatic size cap (see
// internal/cache's package doc: it only ever grows from files the user has
// actually searched, never a background index), so this is the one place a
// user can see it exist and reclaim the disk space.
func (a *App) showAboutDialog() {
	aboutLabel := widget.NewLabel(
		"SearchBoar v. " + version.Version + "\n\n" +
			"SearchBoar was developed for the study of Epicurus as pursued at EpicureanFriends.com",
	)
	aboutLabel.Wrapping = fyne.TextWrapWord

	cacheLabel := widget.NewLabel(a.cacheStatsText())
	clearBtn := widget.NewButton("Clear search cache", func() {
		a.cache.Clear()
		cacheLabel.SetText(a.cacheStatsText())
	})

	content := container.NewVBox(
		aboutLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Search Result Cache", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Text extracted from files you've searched is cached so repeat searches over the same files skip re-reading/re-extracting them. Nothing is cached until you search for it."),
		cacheLabel,
		clearBtn,
	)

	d := dialog.NewCustom("About SearchBoar", "OK", content, a.win)
	d.Resize(fyne.NewSize(440, 360))
	d.Show()
}

func (a *App) cacheStatsText() string {
	rows, bytes, err := a.cache.Stats()
	if err != nil {
		return "Cache: unavailable"
	}
	return fmt.Sprintf("Cache: %d file(s), %s", rows, formatSize(bytes))
}
