package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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
	// Slightly smaller than the toolbar's natural row height so the round
	// logo doesn't crowd/overflow the colored toolbar strip.
	icon.SetMinSize(fyne.NewSize(22, 22))

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
	a.commonTerms = newCommonTermsTab(a)

	builderContent := a.builder.build()
	locationsContent := a.locations.build()
	resultsContent := a.results.build()
	favContent := a.favTab.build()
	favSearchesContent := a.favSearches.build()
	startContent := a.start.build()
	commonTermsContent := a.commonTerms.build()

	// Visual tab order (independent of the build order above). Icons make
	// these read unambiguously as tabs rather than plain text links.
	startItem := container.NewTabItemWithIcon("Start", theme.HomeIcon(), startContent)
	commonTermsItem := container.NewTabItemWithIcon("Common Search Terms", theme.HistoryIcon(), commonTermsContent)
	a.tabs = container.NewAppTabs(
		startItem,
		container.NewTabItemWithIcon("Search Builder", theme.SearchIcon(), builderContent),
		container.NewTabItemWithIcon("Search Locations", theme.StorageIcon(), locationsContent),
		container.NewTabItemWithIcon("Detailed Results", theme.ListIcon(), resultsContent),
		container.NewTabItemWithIcon("Favorite Results", theme.DocumentIcon(), favContent),
		container.NewTabItemWithIcon("Favorite Searches", theme.HistoryIcon(), favSearchesContent),
		commonTermsItem,
	)
	a.tabs.OnSelected = func(item *container.TabItem) {
		if item == startItem {
			a.start.refresh()
		} else if item == commonTermsItem {
			a.commonTerms.refresh()
		}
	}

	a.statusBar = widget.NewLabel("Ready")
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	toolbar := a.buildToolbar()

	// A colored strip behind the toolbar, tied to the same accent color as
	// highlights/selection elsewhere (Settings dialog's Appearance tab) --
	// applyThemeChange keeps this rectangle's fill in sync whenever the
	// accent changes.
	a.toolbarBg = canvas.NewRectangle(a.theme.accent)
	toolbarRow := container.NewStack(a.toolbarBg, toolbar)

	content := container.NewBorder(
		toolbarRow,
		container.NewVBox(a.progressBar, a.statusBar),
		nil, nil,
		a.tabs,
	)
	a.win.SetContent(content)
}

// applyThemeChange persists the theme's current accent color and dark/light
// mode, re-applies the theme (Fyne re-queries every widget's colors when
// the same theme instance is re-set, so this takes effect immediately
// without a restart), and keeps the toolbar background rectangle in sync.
// Called after any change to a.theme's accent or dark/light mode.
func (a *App) applyThemeChange() {
	a.cfg.AccentColor = a.theme.AccentHex()
	if a.theme.dark {
		a.cfg.ThemeMode = ""
	} else {
		a.cfg.ThemeMode = "light"
	}
	a.cfg.Save()
	a.toolbarBg.FillColor = a.theme.accent
	a.toolbarBg.Refresh()
	a.fyneApp.Settings().SetTheme(a.theme)
}

// toggleThemeMode flips between dark and light mode -- the toolbar's
// sun/moon icon's action (see toolbar.go).
func (a *App) toggleThemeMode() {
	a.theme.dark = !a.theme.dark
	a.applyThemeChange()
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
