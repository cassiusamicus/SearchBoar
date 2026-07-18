package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// nordTheme is a dark theme built from the Nord palette
// (https://www.nordtheme.com) -- a softer, blue-gray dark mode instead of
// Fyne's near-black default, per user preference.
type nordTheme struct{}

// Nord palette.
var (
	nord0  = color.NRGBA{0x2E, 0x34, 0x40, 0xFF} // background
	nord1  = color.NRGBA{0x3B, 0x42, 0x52, 0xFF} // buttons, inputs, panels
	nord2  = color.NRGBA{0x43, 0x4C, 0x5E, 0xFF} // hover/pressed/selection
	nord3  = color.NRGBA{0x4C, 0x56, 0x6A, 0xFF} // disabled, borders, placeholder
	nord4  = color.NRGBA{0xD8, 0xDE, 0xE9, 0xFF} // foreground text
	nord6  = color.NRGBA{0xEC, 0xEF, 0xF4, 0xFF} // brightest text
	nord8  = color.NRGBA{0x88, 0xC0, 0xD0, 0xFF} // primary/accent (cyan)
	nord9  = color.NRGBA{0x81, 0xA1, 0xC1, 0xFF} // hyperlink (blue)
	nord11 = color.NRGBA{0xBF, 0x61, 0x6A, 0xFF} // error (red)
	nord13 = color.NRGBA{0xEB, 0xCB, 0x8B, 0xFF} // warning (yellow)
	nord14 = color.NRGBA{0xA3, 0xBE, 0x8C, 0xFF} // success (green)
)

func (nordTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return nord0
	case theme.ColorNameForeground:
		return nord4
	case theme.ColorNameButton, theme.ColorNameDisabledButton, theme.ColorNameInputBackground,
		theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground,
		theme.ColorNameScrollBarBackground:
		return nord1
	case theme.ColorNameHover, theme.ColorNamePressed, theme.ColorNameSelection,
		theme.ColorNameInnerWindowBorder, theme.ColorNameInnerWindowBorderInactive, theme.ColorNameSeparator:
		return nord2
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder, theme.ColorNameInputBorder, theme.ColorNameScrollBar:
		return nord3
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return nord8
	case theme.ColorNameHyperlink:
		return nord9
	case theme.ColorNameForegroundOnPrimary:
		return nord0
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

func (nordTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (nordTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (nordTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
