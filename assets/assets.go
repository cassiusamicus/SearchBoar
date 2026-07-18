// Package assets embeds static resources shared by the SearchBoar application.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconBytes []byte

// Icon returns the SearchBoar application/window icon.
func Icon() fyne.Resource {
	return fyne.NewStaticResource("searchboar-icon.png", iconBytes)
}
