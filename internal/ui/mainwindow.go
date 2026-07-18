package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

// newMainWindow builds the root content of the main window. It currently
// only proves out the window/icon/wordmark scaffold; the toolbar and tabs
// are added in later milestones.
func newMainWindow(a fyne.App, w fyne.Window) fyne.CanvasObject {
	return container.NewBorder(wordmark(), nil, nil, nil)
}
