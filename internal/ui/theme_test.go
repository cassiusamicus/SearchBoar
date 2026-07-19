package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/theme"
)

func TestParseHexColorAcceptsWithAndWithoutHash(t *testing.T) {
	want := color.NRGBA{R: 0x22, G: 0x51, B: 0x67, A: 0xFF}
	for _, s := range []string{"#225167", "225167", " #225167 "} {
		c, ok := parseHexColor(s)
		if !ok {
			t.Errorf("parseHexColor(%q) failed, want success", s)
			continue
		}
		if c != want {
			t.Errorf("parseHexColor(%q) = %+v, want %+v", s, c, want)
		}
	}
}

func TestParseHexColorRejectsInvalid(t *testing.T) {
	for _, s := range []string{"", "#fff", "#gggggg", "225167a", "not a color"} {
		if _, ok := parseHexColor(s); ok {
			t.Errorf("parseHexColor(%q) succeeded, want failure", s)
		}
	}
}

func TestColorToHexRoundTrips(t *testing.T) {
	c := color.NRGBA{R: 0x22, G: 0x51, B: 0x67, A: 0xFF}
	hex := colorToHex(c)
	if hex != "#225167" {
		t.Errorf("colorToHex(%+v) = %q, want %q", c, hex, "#225167")
	}
	back, ok := parseHexColor(hex)
	if !ok || back != c {
		t.Errorf("round trip: parseHexColor(%q) = %+v, %v, want %+v, true", hex, back, ok, c)
	}
}

func TestContrastingTextPicksReadableColor(t *testing.T) {
	darkAccent := color.NRGBA{R: 0x22, G: 0x51, B: 0x67, A: 0xFF} // the app's default accent
	if got := contrastingText(darkAccent); got != nord6 {
		t.Errorf("contrastingText(dark accent) = %+v, want nord6 (light text)", got)
	}

	lightAccent := color.NRGBA{R: 0x88, G: 0xC0, B: 0xD0, A: 0xFF} // the old nord8 default
	if got := contrastingText(lightAccent); got != nord0 {
		t.Errorf("contrastingText(light accent) = %+v, want nord0 (dark text)", got)
	}
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
}

func TestEnsureContrastLightensDarkAccentInDarkMode(t *testing.T) {
	darkAccent := color.NRGBA{R: 0x22, G: 0x51, B: 0x67, A: 0xFF} // the app's default accent, lum ~69
	got := ensureContrast(darkAccent, true)
	if luminance(got) <= luminance(darkAccent) {
		t.Errorf("ensureContrast(dark accent, dark mode) did not lighten: got lum %.1f, original lum %.1f",
			luminance(got), luminance(darkAccent))
	}
	if luminance(got) < 130 {
		t.Errorf("ensureContrast(dark accent, dark mode) = lum %.1f, want >= 130 for legibility", luminance(got))
	}
}

func TestEnsureContrastLeavesAlreadyLightAccentAloneInDarkMode(t *testing.T) {
	lightAccent := color.NRGBA{R: 0x88, G: 0xC0, B: 0xD0, A: 0xFF} // lum well above the threshold
	got := ensureContrast(lightAccent, true)
	if got != color.Color(lightAccent) {
		t.Errorf("ensureContrast(already-light accent, dark mode) = %+v, want unchanged %+v", got, lightAccent)
	}
}

func TestEnsureContrastDarkensLightAccentInLightMode(t *testing.T) {
	lightAccent := color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF} // near-white, lum ~224
	got := ensureContrast(lightAccent, false)
	if luminance(got) >= luminance(lightAccent) {
		t.Errorf("ensureContrast(light accent, light mode) did not darken: got lum %.1f, original lum %.1f",
			luminance(got), luminance(lightAccent))
	}
}

func TestEnsureContrastLeavesAlreadyDarkAccentAloneInLightMode(t *testing.T) {
	darkAccent := color.NRGBA{R: 0x22, G: 0x51, B: 0x67, A: 0xFF} // lum ~69, fine as-is against light panels
	got := ensureContrast(darkAccent, false)
	if got != color.Color(darkAccent) {
		t.Errorf("ensureContrast(dark accent, light mode) = %+v, want unchanged %+v", got, darkAccent)
	}
}

// TestPrimaryAndForegroundOnPrimaryStayReadablePair guards against the
// regression this was written for: the entry cursor, a checked checkbox's
// checkmark, and a HighImportance button's background all use
// ColorNamePrimary directly, and the button additionally pairs it with
// ColorNameForegroundOnPrimary for its text -- both need to resolve
// against the *same* effective color (see effectivePrimary) or a
// HighImportance button ends up with low-contrast text once
// ColorNamePrimary itself is adjusted for legibility.
func TestPrimaryAndForegroundOnPrimaryStayReadablePair(t *testing.T) {
	th := newNordTheme("#225167", true) // the app's actual default: a dark accent, dark mode
	primary := th.Color(theme.ColorNamePrimary, 0)
	fg := th.Color(theme.ColorNameForegroundOnPrimary, 0)
	if delta := luminance(fg) - luminance(primary); delta < 0 {
		delta = -delta
	} else if delta < 80 {
		t.Errorf("Primary/ForegroundOnPrimary luminance delta = %.1f, want a clearly readable pair (>=80)", delta)
	}
}

func TestToolbarIconPairsWithRawAccentNotEffectivePrimary(t *testing.T) {
	// A dark accent in dark mode: effectivePrimary lightens it, but the
	// toolbar's actual background is the raw accent (mainwindow.go's
	// toolbarBg), so colorNameToolbarIcon must be computed against the raw
	// accent, not effectivePrimary, or the icon color would be picked for
	// contrast against a color that isn't what's actually behind it.
	th := newNordTheme("#225167", true)
	iconColor := th.Color(colorNameToolbarIcon, 0)
	want := contrastingText(th.accent)
	if iconColor != want {
		t.Errorf("toolbar icon color = %+v, want contrastingText(raw accent) = %+v", iconColor, want)
	}
}

func TestNewNordThemeDefaultsOnInvalidHex(t *testing.T) {
	th := newNordTheme("not a color", true)
	if th.accent != defaultAccent {
		t.Errorf("accent = %+v, want defaultAccent %+v", th.accent, defaultAccent)
	}
}

func TestNewNordThemeUsesProvidedHex(t *testing.T) {
	th := newNordTheme("#FF8800", true)
	if th.AccentHex() != "#FF8800" {
		t.Errorf("AccentHex() = %q, want %q", th.AccentHex(), "#FF8800")
	}
}
