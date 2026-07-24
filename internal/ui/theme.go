package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// nordTheme is a theme built from the Nord palette
// (https://www.nordtheme.com), which was designed with both a dark mode
// ("Polar Night", nord0-nord3) and a light mode ("Snow Storm", nord4-nord6)
// sharing the same palette -- switching modes here just swaps which end of
// the palette is used for the background/foreground/panel roles, the same
// colors either way.
//
// The accent color (highlights, selection, the toolbar background) is
// user-configurable (Settings dialog's Appearance tab) independent of
// dark/light mode. nordTheme is used as a pointer (not a zero-size value)
// specifically so its accent and dark/light mode can change at runtime and
// be re-applied via app.Settings().SetTheme(sameInstance), which triggers
// Fyne to re-query every widget's colors without restarting the app.
type nordTheme struct {
	accent color.Color
	dark   bool
}

// defaultAccent is used whenever no accent is configured, or a configured
// hex string fails to parse.
var defaultAccent = color.NRGBA{0x22, 0x51, 0x67, 0xFF}

// newNordTheme builds a theme with accentHex as its accent color (e.g.
// "#225167"; an empty or invalid hex string falls back to defaultAccent)
// and dark as its initial light/dark mode.
func newNordTheme(accentHex string, dark bool) *nordTheme {
	t := &nordTheme{accent: defaultAccent, dark: dark}
	t.SetAccentHex(accentHex)
	return t
}

// SetAccentHex parses hex (with or without a leading '#') and sets it as
// the accent color, reporting whether it was valid. An invalid string
// leaves the current accent unchanged.
func (t *nordTheme) SetAccentHex(hex string) bool {
	c, ok := parseHexColor(hex)
	if !ok {
		return false
	}
	t.accent = c
	return true
}

// AccentHex returns the current accent color as "#RRGGBB".
func (t *nordTheme) AccentHex() string {
	return colorToHex(t.accent)
}

func parseHexColor(s string) (color.Color, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return nil, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return nil, false
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}, true
}

func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

// colorNameToolbarIcon is a custom (non-Fyne-builtin) theme color name for
// toolbar icon color specifically. The toolbar's background is filled
// directly from the raw, user-chosen accent (mainwindow.go's toolbarBg),
// not looked up via ColorNamePrimary -- so toolbar icons need their own
// contrast pairing against that raw accent, independent of whatever
// ColorNamePrimary itself resolves to (see effectivePrimary below).
const colorNameToolbarIcon fyne.ThemeColorName = "toolbarIcon"

// effectivePrimary returns t.accent adjusted for legibility when used as
// ColorNamePrimary. Fyne hardcodes several widgets' foreground/interactive
// elements to this color -- the Entry text cursor, a checked Check's
// checkmark icon, a focused Entry's border, a HighImportance button's
// background, radio buttons, progress bar fill -- always drawn against
// that mode's panel/background colors. A deliberately dark accent (picked,
// per an earlier round, so it wouldn't double as unreadable light text
// elsewhere -- see defaultAccent) then reads as nearly invisible in
// exactly those spots: a dark checkmark on a dark checkbox, a dark cursor
// on a dark input background. Nudged toward white in dark mode (toward
// black in light mode), but only when the accent's own luminance is
// already too close to that mode's panels to give reasonable contrast, so
// it keeps the same hue and still reads as "the chosen accent." Literal
// uses of the accent -- the toolbar background fill, the Settings color
// swatch -- go through t.accent directly and are unaffected by this.
func (t *nordTheme) effectivePrimary() color.Color {
	return ensureContrast(t.accent, t.dark)
}

// ensureContrast lightens c toward white (dark mode) or darkens it toward
// black (light mode) when its own luminance is too close to that mode's
// panel colors to be legible as a small foreground element, leaving it
// unchanged otherwise.
func ensureContrast(c color.Color, dark bool) color.Color {
	r, g, b, a := c.RGBA()
	r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)
	a8 := uint8(a >> 8)
	lum := 0.299*r8 + 0.587*g8 + 0.114*b8
	const (
		darkModeMinLum  = 130 // below this, a dark accent blends into dark-mode panels
		lightModeMaxLum = 150 // above this, a light accent blends into light-mode panels
		blend           = 0.55
	)
	switch {
	case dark && lum < darkModeMinLum:
		return color.NRGBA{
			R: uint8(r8 + (255-r8)*blend),
			G: uint8(g8 + (255-g8)*blend),
			B: uint8(b8 + (255-b8)*blend),
			A: a8,
		}
	case !dark && lum > lightModeMaxLum:
		return color.NRGBA{
			R: uint8(r8 * (1 - blend)),
			G: uint8(g8 * (1 - blend)),
			B: uint8(b8 * (1 - blend)),
			A: a8,
		}
	}
	return c
}

// contrastingText picks nord0 (near-black) or nord6 (near-white), whichever
// contrasts better against bg -- used for text/icons drawn on top of the
// user-configurable accent color, which might end up light (needing dark
// text on it) or dark (needing light text), unlike every other color pair
// in this theme, which is fixed and was picked by hand for contrast.
func contrastingText(bg color.Color) color.Color {
	r, g, b, _ := bg.RGBA()
	// Perceived luminance (ITU-R BT.601), 0-255 range from 16-bit RGBA.
	lum := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
	if lum > 140 {
		return nord0
	}
	return nord6
}

// Nord palette. nord0-nord3 ("Polar Night") are the dark end; nord4-nord6
// ("Snow Storm") are the light end; light mode uses them in reverse roles
// from dark mode (see Color below). The accent is configurable -- see
// nordTheme.accent above -- and not part of this fixed palette.
var (
	nord0  = color.NRGBA{0x2E, 0x34, 0x40, 0xFF}
	nord1  = color.NRGBA{0x3B, 0x42, 0x52, 0xFF}
	nord2  = color.NRGBA{0x43, 0x4C, 0x5E, 0xFF}
	nord3  = color.NRGBA{0x4C, 0x56, 0x6A, 0xFF}
	nord4  = color.NRGBA{0xD8, 0xDE, 0xE9, 0xFF}
	nord5  = color.NRGBA{0xE5, 0xE9, 0xF0, 0xFF}
	nord6  = color.NRGBA{0xEC, 0xEF, 0xF4, 0xFF}
	nord9  = color.NRGBA{0x81, 0xA1, 0xC1, 0xFF} // hyperlink (blue)
	nord11 = color.NRGBA{0xBF, 0x61, 0x6A, 0xFF} // error (red)
	nord13 = color.NRGBA{0xEB, 0xCB, 0x8B, 0xFF} // warning (yellow)
	nord14 = color.NRGBA{0xA3, 0xBE, 0x8C, 0xFF} // success (green)

	// nordPlaceholderDark is nord3 blended about a third of the way toward
	// nord4 (dark mode's real foreground) -- see ColorNameDisabled/
	// ColorNamePlaceHolder's own comment in Color below for why nord3
	// alone isn't legible here. Shared by both color names: a disabled
	// Entry showing real text (e.g. the Start tab's File Types summary
	// field) and a blank Entry's placeholder text have the exact same
	// legibility problem against this theme's dark backgrounds.
	nordPlaceholderDark = color.NRGBA{0x7D, 0x86, 0x96, 0xFF}
)

func (t *nordTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		if t.dark {
			return nord0
		}
		return nord6
	case theme.ColorNameForeground:
		if t.dark {
			return nord4
		}
		return nord0
	case theme.ColorNameButton, theme.ColorNameDisabledButton, theme.ColorNameInputBackground,
		theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground,
		theme.ColorNameScrollBarBackground:
		if t.dark {
			return nord1
		}
		return nord5
	case theme.ColorNameHover, theme.ColorNamePressed,
		theme.ColorNameInnerWindowBorder, theme.ColorNameInnerWindowBorderInactive, theme.ColorNameSeparator:
		if t.dark {
			return nord2
		}
		return nord4
	case theme.ColorNameInputBorder, theme.ColorNameScrollBar:
		return nord3 // works as a mid-tone border shade in both modes -- not text, so it doesn't need Disabled/PlaceHolder's legibility floor below
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		if t.dark {
			// nord3 alone is close enough in luminance to nord0/nord1
			// (this theme's dark backgrounds) that placeholder text --
			// "Text or regex to search for in files", "All types", etc.
			// -- and disabled-but-still-meaningful text -- a disabled
			// Entry showing "ORG" or "All types" (the Start tab's File
			// Types summary field), not just an inactive button -- read
			// as genuinely unreadable, not just subtly de-emphasized.
			// Lightened partway toward nord4 (this mode's real
			// foreground) for the same reason effectivePrimary lightens
			// a too-dark accent, while staying dim enough to still read
			// as de-emphasized rather than regular entered text.
			return nordPlaceholderDark
		}
		return nord3
	case theme.ColorNamePrimary:
		return t.effectivePrimary()
	case colorNameToolbarIcon:
		// Paired with the toolbar's actual (raw, unadjusted) accent
		// background -- see the constant's own comment.
		return contrastingText(t.accent)
	case theme.ColorNameFocus:
		// Distinct from Primary (the accent): a Check widget's checked-icon
		// and its focus ring both use Primary/Focus respectively, and with
		// both the same color, a just-clicked (and therefore focused)
		// checked checkbox rendered as one solid-looking circle instead of
		// a square checkbox with a subtle ring around it.
		return nord3
	case theme.ColorNameSelection:
		// Distinct from Hover: List/Tree selection and hover both used the
		// same color, so paging through search results with Prev/Next --
		// which selects a row without the mouse ever hovering it --
		// produced no visually obvious "this one" cue.
		//
		// effectivePrimary, not the raw accent: this same color also backs
		// Entry/RichText's own text-selection highlight (see Fyne's
		// widget/selectable.go), and a low-alpha rectangle drawn from a
		// dark accent (defaultAccent, e.g.) blended into this theme's dark
		// input backgrounds closely enough that a double-click or
		// click-drag selection existed but couldn't actually be seen --
		// the same dark-accent-on-dark-panel problem effectivePrimary
		// already exists to solve for every other accent-derived color.
		r, g, b, _ := t.effectivePrimary().RGBA()
		return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0x50}
	case theme.ColorNameHyperlink:
		return nord9
	case theme.ColorNameForegroundOnPrimary:
		// Paired with effectivePrimary (not the raw accent): a
		// HighImportance button's background is ColorNamePrimary, which is
		// effectivePrimary, not t.accent -- pairing this against the raw
		// accent instead would recompute contrast for a color that isn't
		// actually what's being drawn on, and could pick e.g. light text
		// for a background that effectivePrimary already lightened.
		return contrastingText(t.effectivePrimary())
	case theme.ColorNameError:
		return nord11
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return nord0
	case theme.ColorNameWarning:
		return nord13
	case theme.ColorNameSuccess:
		return nord14
	case theme.ColorNameShadow:
		return color.NRGBA{0x00, 0x00, 0x00, 0x50}
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t *nordTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *nordTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *nordTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameHeadingText {
		// The default theme's card-title size (24) reads oversized against
		// this app's 14pt body text -- every widget.Card in the app (Quick
		// Search, Quick Location, Search In, Storage Locations, etc.) uses
		// this size for its title, so shrinking it here fixes all of them
		// at once rather than needing a per-card workaround.
		return 16
	}
	return theme.DefaultTheme().Size(name)
}
