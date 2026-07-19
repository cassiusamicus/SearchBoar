package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Fyne's built-in icon set (theme.*Icon()) has nothing resembling a sun or
// moon, so the dark/light toggle button uses two small inline SVGs instead.
// theme.NewThemedResource recolors an SVG's fill to the current theme
// color at render time (the same mechanism every built-in icon uses) --
// but it (or the underlying oksvg rasterizer Fyne uses) only reliably
// renders <path> elements the way Fyne's own bundled icons are built
// (confirmed by inspecting theme/icons/*.svg); an earlier version of this
// icon used <circle>/<line>/<g>, which silently rendered as a plain dot.
// Rays are drawn as simple axis-aligned triangles instead of a stroked
// line, and the sun's disc/moon's crescent are drawn via SVG arc (A)
// commands, matching what Fyne's own icons already use successfully.
const sunSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path d="M12,7.5 A4.5,4.5 0 1,0 12,16.5 A4.5,4.5 0 1,0 12,7.5 Z"/>
<path d="M10.5,7 L13.5,7 L12,2 Z"/>
<path d="M10.5,17 L13.5,17 L12,22 Z"/>
<path d="M17,10.5 L17,13.5 L22,12 Z"/>
<path d="M7,10.5 L7,13.5 L2,12 Z"/>
</svg>`

const moonSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path d="M12 3a9 9 0 1 0 9 9 7 7 0 0 1-9-9z"/>
</svg>`

func sunIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("sun.svg", []byte(sunSVG)))
}

func moonIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("moon.svg", []byte(moonSVG)))
}
