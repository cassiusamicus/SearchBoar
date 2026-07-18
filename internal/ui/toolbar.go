package ui

import (
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/version"
)

const (
	tabIndexLocations = iota
	tabIndexBuilder
	tabIndexResults
	tabIndexFavorites
)

func (a *App) buildToolbar() *widget.Toolbar {
	a.searchButton = widget.NewToolbarAction(theme.SearchIcon(), func() { a.startSearch() })
	a.stopButton = widget.NewToolbarAction(theme.MediaStopIcon(), func() { a.stopSearch() })
	a.stopButton.Disable()

	t := widget.NewToolbar(
		toolbarWidgetItem{obj: wordmark()},
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.HomeIcon(), func() { a.tabs.SelectIndex(tabIndexLocations) }),
		a.searchButton,
		widget.NewToolbarAction(theme.DocumentIcon(), func() { a.showStoredSearchesDialog() }),
		widget.NewToolbarAction(theme.ListIcon(), func() { a.tabs.SelectIndex(tabIndexFavorites) }),
		widget.NewToolbarSeparator(),
		a.stopButton,
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.SettingsIcon(), func() { a.openConfigFile() }),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.HelpIcon(), func() { a.showAboutDialog() }),
	)
	return t
}

func (a *App) showAboutDialog() {
	dialog.ShowInformation(
		"About SearchBoar",
		"SearchBoar v. "+version.Version+"\n\n"+
			"SearchBoar was developed for the study of Epicurus as pursued at EpicureanFriends.com",
		a.win,
	)
}
