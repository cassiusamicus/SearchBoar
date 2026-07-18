package ui

import (
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
	a.searchButton = newIconTipButton(theme.SearchIcon(), "Start a search", a.win, func() { a.startSearch() })
	a.stopButton = newIconTipButton(theme.MediaStopIcon(), "Stop the current search", a.win, func() { a.stopSearch() })
	a.stopButton.Disable()

	// A stretchable spacer keeps the wordmark and the icon group apart
	// instead of crowding them together, and every icon gets a hover
	// tooltip (see icontip.go -- Fyne has no built-in tooltip widget) since
	// several of these (Favorite Searches vs. Favorites, Stop) aren't
	// self-explanatory from the icon alone.
	t := widget.NewToolbar(
		toolbarWidgetItem{obj: wordmark()},
		widget.NewToolbarSpacer(),
		toolbarWidgetItem{obj: newIconTipButton(theme.HomeIcon(), "Start tab", a.win, func() { a.tabs.SelectIndex(tabIndexStart) })},
		toolbarWidgetItem{obj: a.searchButton},
		toolbarWidgetItem{obj: a.stopButton},
		toolbarWidgetItem{obj: newIconTipButton(theme.DocumentIcon(), "Favorite Searches", a.win, func() { a.tabs.SelectIndex(tabIndexFavSearches) })},
		toolbarWidgetItem{obj: newIconTipButton(theme.ListIcon(), "Favorite Results", a.win, func() { a.tabs.SelectIndex(tabIndexFavorites) })},
		toolbarWidgetItem{obj: newIconTipButton(theme.SettingsIcon(), "Settings", a.win, func() { a.showSettingsDialog() })},
		toolbarWidgetItem{obj: newIconTipButton(theme.HelpIcon(), "About SearchBoar", a.win, func() { a.showAboutDialog() })},
	)
	return t
}

func (a *App) showAboutDialog() {
	aboutLabel := widget.NewLabel(
		"SearchBoar v. " + version.Version + "\n\n" +
			"SearchBoar was developed for the study of Epicurus as pursued at EpicureanFriends.com",
	)
	aboutLabel.Wrapping = fyne.TextWrapWord

	d := dialog.NewCustom("About SearchBoar", "OK", container.NewVBox(aboutLabel), a.win)
	d.Resize(fyne.NewSize(400, 200))
	d.Show()
}
