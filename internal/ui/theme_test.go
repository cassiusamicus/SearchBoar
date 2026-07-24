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

// TestPlaceHolderReadableAgainstDarkInputBackground guards the reported
// bug: placeholder text ("Text or regex to search for in files", "All
// types", etc.) was nord3 in every mode, close enough in luminance to
// nord1 (this theme's dark input background) to be genuinely unreadable
// in dark mode, not just subtly de-emphasized.
func TestPlaceHolderReadableAgainstDarkInputBackground(t *testing.T) {
	th := newNordTheme("#225167", true)
	placeholder := th.Color(theme.ColorNamePlaceHolder, 0)
	inputBg := th.Color(theme.ColorNameInputBackground, 0)
	if delta := luminance(placeholder) - luminance(inputBg); delta < 40 {
		t.Errorf("placeholder/input-background luminance delta = %.1f, want >= 40 for legibility", delta)
	}
	// Still dimmer than real foreground text, so it keeps reading as a
	// placeholder rather than entered text.
	fg := th.Color(theme.ColorNameForeground, 0)
	if luminance(placeholder) >= luminance(fg) {
		t.Errorf("placeholder luminance %.1f >= foreground luminance %.1f, want placeholder still dimmer",
			luminance(placeholder), luminance(fg))
	}
}

func TestPlaceHolderUnchangedInLightMode(t *testing.T) {
	th := newNordTheme("#225167", false)
	if got := th.Color(theme.ColorNamePlaceHolder, 0); got != color.Color(nord3) {
		t.Errorf("light-mode placeholder = %+v, want nord3 %+v (already legible against light panels)", got, nord3)
	}
}

// TestDisabledReadableAgainstDarkInputBackground guards the follow-up
// report: a disabled Entry showing real text (the Start tab's File Types
// summary field, e.g. "ORG") is ColorNameDisabled, not ColorNamePlaceHolder
// -- a separate color name with the exact same nord3-too-dark problem, so
// fixing only PlaceHolder left this one just as unreadable.
func TestDisabledReadableAgainstDarkInputBackground(t *testing.T) {
	th := newNordTheme("#225167", true)
	disabled := th.Color(theme.ColorNameDisabled, 0)
	inputBg := th.Color(theme.ColorNameInputBackground, 0)
	if delta := luminance(disabled) - luminance(inputBg); delta < 40 {
		t.Errorf("disabled/input-background luminance delta = %.1f, want >= 40 for legibility", delta)
	}
	fg := th.Color(theme.ColorNameForeground, 0)
	if luminance(disabled) >= luminance(fg) {
		t.Errorf("disabled luminance %.1f >= foreground luminance %.1f, want disabled still dimmer",
			luminance(disabled), luminance(fg))
	}
}

func TestDisabledUnchangedInLightMode(t *testing.T) {
	th := newNordTheme("#225167", false)
	if got := th.Color(theme.ColorNameDisabled, 0); got != color.Color(nord3) {
		t.Errorf("light-mode disabled = %+v, want nord3 %+v (already legible against light panels)", got, nord3)
	}
}

// TestDisabledAndPlaceHolderMatch guards the two color names staying in
// sync: they're deliberately the same fix for the same underlying
// legibility problem (see nordPlaceholderDark's own comment), so a future
// edit to one that forgets the other would silently reintroduce this bug
// for whichever one got missed.
func TestDisabledAndPlaceHolderMatch(t *testing.T) {
	for _, dark := range []bool{true, false} {
		th := newNordTheme("#225167", dark)
		disabled := th.Color(theme.ColorNameDisabled, 0)
		placeholder := th.Color(theme.ColorNamePlaceHolder, 0)
		if disabled != placeholder {
			t.Errorf("dark=%v: Disabled = %+v, PlaceHolder = %+v, want them equal", dark, disabled, placeholder)
		}
	}
}

// TestSelectionUsesEffectivePrimaryNotRawAccent guards the other reported
// bug: a double-click or click-drag text selection inside an Entry (Fyne's
// widget/selectable.go also draws from ColorNameSelection) was invisible
// in dark mode because the raw default accent is dark enough to blend
// into this theme's dark input backgrounds even before the selection
// rectangle's own alpha is applied. Compares the stored color.NRGBA
// fields directly, not via .RGBA() -- that method alpha-premultiplies its
// result, and a straight comparison against effectivePrimary's (opaque,
// so premultiplication is a no-op there) RGBA() output would misreport a
// mismatch that isn't really there.
func TestSelectionUsesEffectivePrimaryNotRawAccent(t *testing.T) {
	th := newNordTheme("#225167", true) // dark accent, dark mode -- effectivePrimary lightens it
	selection := th.Color(theme.ColorNameSelection, 0)
	sel, ok := selection.(color.NRGBA)
	if !ok {
		t.Fatalf("selection color = %T, want color.NRGBA", selection)
	}

	pr, pg, pb, _ := th.effectivePrimary().RGBA()
	wantR, wantG, wantB := uint8(pr>>8), uint8(pg>>8), uint8(pb>>8)
	if sel.R != wantR || sel.G != wantG || sel.B != wantB {
		t.Errorf("selection RGB = (%d,%d,%d), want effectivePrimary's RGB (%d,%d,%d), not the raw accent",
			sel.R, sel.G, sel.B, wantR, wantG, wantB)
	}
	if sel.A != 0x50 {
		t.Errorf("selection alpha = %#x, want 0x50", sel.A)
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
