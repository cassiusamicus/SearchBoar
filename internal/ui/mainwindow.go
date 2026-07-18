package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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

func wordmark() fyne.CanvasObject {
	search := canvas.NewText("Search", brandBlue)
	search.TextStyle = fyne.TextStyle{Bold: true}
	search.TextSize = 22

	boar := canvas.NewText("Boar", brandOrange)
	boar.TextStyle = fyne.TextStyle{Bold: true}
	boar.TextSize = 22

	return container.NewHBox(search, boar)
}

func (a *App) buildMainWindow() {
	a.basic = newBasicTab(a)
	a.advanced = newAdvancedTab(a)
	a.details = newDetailsTab(a)
	a.overview = newOverviewTab(a)
	a.favTab = newFavoritesTab(a)

	a.tabs = container.NewAppTabs(
		container.NewTabItem("Basic", a.basic.build()),
		container.NewTabItem("Advanced", a.advanced.build()),
		container.NewTabItem("Result Details", a.details.build()),
		container.NewTabItem("Result Overview", a.overview.build()),
		container.NewTabItem("Favorite Results", a.favTab.build()),
	)

	a.statusBar = widget.NewLabel("Ready")
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	toolbar := a.buildToolbar()

	content := container.NewBorder(
		container.NewVBox(toolbar, wordmark()),
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
