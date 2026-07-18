// Package ui contains all Fyne GUI code for SearchBoar. It is the only
// package in this module that imports fyne.io/fyne/v2; everything it wires
// together (search engine, config, favorites, network search) is UI-agnostic
// and independently testable.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"codeberg.org/cassiusamicus/Utilities/assets"
)

// Run builds and shows the main SearchBoar window, then blocks until the
// window is closed.
func Run() {
	a := app.NewWithID("com.epicureanfriends.searchboar")
	a.SetIcon(assets.Icon())

	w := a.NewWindow("SearchBoar")
	w.SetIcon(assets.Icon())
	w.Resize(fyne.NewSize(900, 600))

	w.SetContent(newMainWindow(a, w))

	w.ShowAndRun()
}
