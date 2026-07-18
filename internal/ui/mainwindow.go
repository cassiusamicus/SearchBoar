package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/assets"
)

// brand colors carried forward from the original GTK3 app's wordmark and
// match-highlight styling.
var (
	brandBlue   = &fyneColor{r: 0x5D, g: 0xAD, b: 0xE2, a: 0xFF}
	brandOrange = &fyneColor{r: 0xE6, g: 0x7E, b: 0x22, a: 0xFF}
)

type fyneColor struct{ r, g, b, a uint8 }

func (c *fyneColor) RGBA() (r, g, b, a uint32) {
	return uint32(c.r) * 0x101, uint32(c.g) * 0x101, uint32(c.b) * 0x101, uint32(c.a) * 0x101
}

// wordmark is the logo image plus "Search"/"Boar" text, rendered as a
// single toolbar item so it sits on the same row as the other toolbar
// icons rather than as a separate banner row.
func wordmark() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(assets.Icon())
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(28, 28))

	search := canvas.NewText("Search", brandBlue)
	search.TextStyle = fyne.TextStyle{Bold: true}
	search.TextSize = 20

	boar := canvas.NewText("Boar", brandOrange)
	boar.TextStyle = fyne.TextStyle{Bold: true}
	boar.TextSize = 20

	return container.NewHBox(icon, search, boar)
}

// toolbarWidgetItem adapts an arbitrary CanvasObject (the wordmark) into a
// widget.ToolbarItem, since Toolbar only ships action/separator/spacer
// item types.
type toolbarWidgetItem struct{ obj fyne.CanvasObject }

func (t toolbarWidgetItem) ToolbarObject() fyne.CanvasObject { return t.obj }

func (a *App) buildMainWindow() {
	// Constructed (and built) in dependency order: startTab.build() reads
	// widget state from builder/locations, so those must already be built.
	a.builder = newSearchBuilderTab(a)
	a.locations = newLocationsTab(a)
	a.results = newResultsTab(a)
	a.favTab = newFavoritesTab(a)
	a.favSearches = newFavoriteSearchesTab(a)
	a.start = newStartTab(a)

	builderContent := a.builder.build()
	locationsContent := a.locations.build()
	resultsContent := a.results.build()
	favContent := a.favTab.build()
	favSearchesContent := a.favSearches.build()
	startContent := a.start.build()

	// Visual tab order (independent of the build order above).
	startItem := container.NewTabItem("Start", startContent)
	a.tabs = container.NewAppTabs(
		startItem,
		container.NewTabItem("Search Builder", builderContent),
		container.NewTabItem("Search Locations", locationsContent),
		container.NewTabItem("Results", resultsContent),
		container.NewTabItem("Favorite Results", favContent),
		container.NewTabItem("Favorite Searches", favSearchesContent),
	)
	a.tabs.OnSelected = func(item *container.TabItem) {
		if item == startItem {
			a.start.refresh()
		}
	}

	a.statusBar = widget.NewLabel("Ready")
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	toolbar := a.buildToolbar()

	content := container.NewBorder(
		toolbar,
		container.NewVBox(a.progressBar, a.statusBar),
		nil, nil,
		a.tabs,
	)
	a.win.SetContent(content)
}

func (a *App) restoreWindowGeometry() {
	w, h := a.cfg.Window.Width, a.cfg.Window.Height
	if w <= 0 || h <= 0 {
		a.win.Resize(defaultWindowSize())
		return
	}
	a.win.Resize(fyne.NewSize(float32(w), float32(h)))
	// Fyne's Window interface has no cross-platform way to set screen
	// position (only size), so the original app's saved x/y is
	// intentionally not restored here -- a platform limitation, not an
	// oversight.
}

func (a *App) saveWindowGeometry() {
	size := a.win.Canvas().Size()
	a.cfg.Window.Width = int(size.Width)
	a.cfg.Window.Height = int(size.Height)
}
