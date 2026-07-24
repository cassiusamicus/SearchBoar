package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/assets"
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
	// widget state from locations, so that must already be built.
	a.locations = newLocationsTab(a)
	a.results = newResultsTab(a)
	a.favTab = newFavoritesTab(a)
	a.favSearches = newFavoriteSearchesTab(a)
	a.start = newStartTab(a)
	a.commonTerms = newCommonTermsTab(a)

	locationsContent := a.locations.build()
	resultsContent := a.results.build()
	favContent := a.favTab.build()
	favSearchesContent := a.favSearches.build()
	startContent := a.start.build()
	commonTermsContent := a.commonTerms.build()

	// Visual tab order (independent of the build order above). Icons make
	// these read unambiguously as tabs rather than plain text links. There
	// used to be a separate "Search Builder" tab here, between Start and
	// Workspace Builder -- its fields now live directly on the Start tab
	// (see starttab.go's own doc comment), so it's gone.
	startItem := container.NewTabItemWithIcon("Start", theme.HomeIcon(), startContent)
	commonTermsItem := container.NewTabItemWithIcon("Common Search Terms", theme.HistoryIcon(), commonTermsContent)
	a.tabs = container.NewAppTabs(
		startItem,
		container.NewTabItemWithIcon("Workspace Builder", theme.StorageIcon(), locationsContent),
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
	// Status text often includes a full file path (search errors, "Deleted
	// /a/very/long/path", etc.) -- without wrapping, one long status
	// message is enough to force the whole window wider than the screen,
	// the same root cause fixed in resultsview.go's result cards.
	a.statusBar.Wrapping = fyne.TextWrapBreak
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()
	a.bottomStopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() { a.stopSearch() })
	a.bottomStopBtn.Hide()
	// Left-aligned at its own natural width, not stretched to the window's
	// full width: a progress bar reads as a bar precisely because it's a
	// bounded track that fills up, and stretching it edge to edge on every
	// launch (even before a search has run) exaggerated it into a big,
	// mostly-empty-looking strip for no reason -- HBox gives it just
	// enough room for itself and the Stop button, with the spacer
	// absorbing the rest instead of the progress bar itself.
	progressRow := container.NewHBox(a.progressBar, a.bottomStopBtn, layout.NewSpacer())

	toolbar := a.buildToolbar()

	// A colored strip behind the toolbar, tied to the same accent color as
	// highlights/selection elsewhere (Settings dialog's Appearance tab) --
	// applyThemeChange keeps this rectangle's fill in sync whenever the
	// accent changes.
	a.toolbarBg = canvas.NewRectangle(a.theme.accent)
	toolbarRow := container.NewStack(a.toolbarBg, toolbar)

	content := container.NewBorder(
		toolbarRow,
		container.NewVBox(progressRow, a.statusBar),
		nil, nil,
		a.tabs,
	)
	a.win.SetContent(content)
}

// applyThemeChange persists the theme's current accent color and dark/light
// mode, re-applies the theme, and keeps the toolbar background rectangle in
// sync. Called after any change to a.theme's accent or dark/light mode.
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
	for _, backdrop := range a.boxedCardBackdrops {
		backdrop.FillColor = boxedCardBackdropColor(a)
		backdrop.Refresh()
	}
	a.fyneApp.Settings().SetTheme(a.theme)
	// Settings.SetTheme notifies listeners over a channel that's a
	// non-blocking send with a goroutine fallback (see Fyne's
	// app/settings.go apply()) -- if that channel isn't immediately ready,
	// the notification (and the resulting widget recolor) lands on some
	// later tick instead of this one, not necessarily before the current
	// frame renders. The visible symptom was the dark/light toggle needing
	// two clicks: the first click's redraw only actually caught up once
	// the second click's own frame rendered. Refreshing the whole content
	// tree here forces every widget to redraw against the new theme
	// immediately, without waiting on that notification to arrive.
	a.win.Content().Refresh()
}

// toggleThemeMode flips between dark and light mode -- the toolbar's
// sun/moon icon's action (see toolbar.go).
func (a *App) toggleThemeMode() {
	a.theme.dark = !a.theme.dark
	a.applyThemeChange()
}

// maxReasonableWindowWidth/Height cap the geometry restored from a
// previous session. Fyne has no portable way to query the actual screen
// size to size against, so this is a conservative "should fit on nearly
// any real monitor" ceiling -- a backstop against a saved size that came
// from a larger/different display, or (before the content-driven-width
// fixes in resultsview.go/mainwindow.go) got inflated by a long path
// forcing the window wider than intended and then saved back to config on
// close, which would otherwise keep reopening too wide forever even after
// the underlying cause was fixed.
const (
	maxReasonableWindowWidth  = 1600
	maxReasonableWindowHeight = 1000
)

func (a *App) restoreWindowGeometry() {
	w, h := a.cfg.Window.Width, a.cfg.Window.Height
	if w <= 0 || h <= 0 {
		a.win.Resize(defaultWindowSize())
		return
	}
	if w > maxReasonableWindowWidth {
		w = maxReasonableWindowWidth
	}
	if h > maxReasonableWindowHeight {
		h = maxReasonableWindowHeight
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
