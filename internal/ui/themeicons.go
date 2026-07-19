package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Fyne's built-in icon set (theme.*Icon()) has nothing resembling a sun or
// moon, so the dark/light toggle button uses two small inline SVGs instead.
// theme.NewThemedResource recolors an SVG's fill to the current theme
// color at render time (the same mechanism every built-in icon uses) --
// but it (or rather the underlying oksvg/rasterx rasterizer Fyne uses)
// only reliably renders <path> elements the way Fyne's own bundled icons
// are built (confirmed by inspecting theme/icons/*.svg); an earlier
// version of this icon used <circle>/<line>/<g>, which silently rendered
// as a plain dot. Rays are drawn as simple axis-aligned triangles instead
// of a stroked line, and the sun's disc is drawn via SVG arc (A) commands
// -- verified correct by rendering it directly through the oksvg/rasterx
// packages Fyne actually uses (not just a general-purpose renderer like
// rsvg-convert, which can legitimately disagree with Fyne's own output).
const sunSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path d="M12,7.5 A4.5,4.5 0 1,0 12,16.5 A4.5,4.5 0 1,0 12,7.5 Z"/>
<path d="M10.5,7 L13.5,7 L12,2 Z"/>
<path d="M10.5,17 L13.5,17 L12,22 Z"/>
<path d="M17,10.5 L17,13.5 L22,12 Z"/>
<path d="M7,10.5 L7,13.5 L2,12 Z"/>
</svg>`

// The moon crescent is a single closed polygon (line segments only, no arc
// commands) tracing the actual crescent outline -- the visible arc of a
// big circle, then the concave arc of a smaller offset circle back to the
// start -- rather than the more common "two overlapping circular arcs"
// construction most crescent icons (Feather's included) use. That
// construction relies on the rasterizer resolving the self-intersecting
// combined path via a nonzero/evenodd winding rule the way a general SVG
// renderer does; oksvg/rasterx (what Fyne actually renders icons with)
// doesn't, at least not for this shape -- verified by rendering it
// directly through those packages: it came out either as a circle with a
// flat concave bite ("Pac-Man") or, with fill-rule="evenodd" added, as a
// distorted double-circle with a visible seam, in both cases wrong, even
// though a general-purpose renderer like rsvg-convert draws the exact
// same path correctly. Tracing the crescent as one non-self-intersecting
// polygon sidesteps the winding-rule question entirely and renders
// correctly through oksvg/rasterx.
const moonSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path d="M11.85,3.00 L10.53,3.12 L9.24,3.43 L8.01,3.93 L6.86,4.61 L5.83,5.44 L4.94,6.42 L4.19,7.52 L3.62,8.72 L3.23,9.99 L3.03,11.30 L3.02,12.63 L3.21,13.95 L3.59,15.22 L4.16,16.42 L4.90,17.53 L5.79,18.51 L6.81,19.35 L7.95,20.04 L9.18,20.55 L10.47,20.87 L11.79,21.00 L13.11,20.93 L14.42,20.67 L15.67,20.22 L16.84,19.59 L17.90,18.80 L18.84,17.85 L19.62,16.78 L20.24,15.61 L20.69,14.36 L20.07,14.75 L19.41,15.08 L18.73,15.35 L18.02,15.54 L17.29,15.66 L16.56,15.70 L15.83,15.67 L15.10,15.56 L14.39,15.38 L13.70,15.13 L13.04,14.81 L12.41,14.43 L11.83,13.98 L11.30,13.48 L10.82,12.92 L10.40,12.32 L10.04,11.68 L9.75,11.00 L9.53,10.30 L9.38,9.58 L9.31,8.85 L9.31,8.12 L9.39,7.39 L9.54,6.67 L9.76,5.97 L10.05,5.30 L10.41,4.66 L10.83,4.06 L11.32,3.50 L11.85,3.00 Z"/>
</svg>`

func sunIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("sun.svg", []byte(sunSVG)))
}

func moonIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("moon.svg", []byte(moonSVG)))
}
