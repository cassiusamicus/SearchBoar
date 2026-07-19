package ui

import (
	"image/color"
	"testing"
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
