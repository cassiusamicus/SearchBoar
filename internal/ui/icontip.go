package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// iconTipButton is a toolbar-style icon button that shows a small popup
// with its tooltip text after a short hover delay. Fyne has no built-in
// tooltip widget (as of v2.8), and wrapping a stock widget.Button in a
// separate Hoverable container doesn't work -- Button already implements
// desktop.Hoverable itself for its own hover highlight, and Fyne only
// dispatches hover events to the deepest Hoverable under the pointer, so
// an outer wrapper's MouseIn/MouseOut would never fire (the same
// dispatch-to-deepest-widget issue that broke the Results tab's list rows
// earlier). Building the hover highlight and the tooltip into one widget
// sidesteps that.
type iconTipButton struct {
	widget.DisableableWidget

	icon    fyne.Resource
	tooltip string
	win     fyne.Window
	onTap   func()

	background *canvas.Rectangle
	iconObj    *widget.Icon

	popUp *widget.PopUp
	timer *time.Timer
}

// newIconTipButton is only ever used for toolbar icons, which sit on the
// toolbar's accent-colored background (see mainwindow.go's toolbarBg) --
// icon is recolored to colorNameToolbarIcon (a contrast-aware color paired
// specifically with that raw accent background, not ColorNameForegroundOnPrimary
// -- that one's paired with effectivePrimary, an adjusted-for-legibility
// variant of the accent used elsewhere, which isn't what's actually behind
// these icons) rather than the general theme foreground, which flips
// dark/light with the overall theme and would go dark-on-dark against a
// dark accent whenever the user is in light mode.
func newIconTipButton(icon fyne.Resource, tooltip string, win fyne.Window, onTap func()) *iconTipButton {
	b := &iconTipButton{icon: onPrimaryIcon(icon), tooltip: tooltip, win: win, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func onPrimaryIcon(icon fyne.Resource) fyne.Resource {
	return theme.NewColoredResource(icon, colorNameToolbarIcon)
}

func (b *iconTipButton) CreateRenderer() fyne.WidgetRenderer {
	th := b.Theme()
	b.background = canvas.NewRectangle(color.Transparent)
	b.background.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	b.iconObj = widget.NewIcon(b.icon)

	content := container.NewStack(b.background, container.NewPadded(b.iconObj))
	return widget.NewSimpleRenderer(content)
}

// SetIcon changes the button's icon and tooltip in place (used by the
// dark/light toggle, which shows a sun in dark mode and a moon in light
// mode -- the icon representing the mode you'd switch to).
func (b *iconTipButton) SetIcon(icon fyne.Resource, tooltip string) {
	b.icon = onPrimaryIcon(icon)
	b.tooltip = tooltip
	if b.iconObj != nil {
		b.iconObj.Resource = b.icon
		b.iconObj.Refresh()
	}
}

func (b *iconTipButton) Tapped(*fyne.PointEvent) {
	if b.Disabled() {
		return
	}
	b.hideTip()
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *iconTipButton) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (b *iconTipButton) MouseIn(*desktop.MouseEvent) {
	if b.Disabled() {
		return
	}
	if b.background != nil {
		v := fyne.CurrentApp().Settings().ThemeVariant()
		b.background.FillColor = b.Theme().Color(theme.ColorNameHover, v)
		b.background.Refresh()
	}
	b.timer = time.AfterFunc(500*time.Millisecond, func() { runOnUI(b.showTip) })
}

func (b *iconTipButton) MouseMoved(*desktop.MouseEvent) {}

func (b *iconTipButton) MouseOut() {
	if b.background != nil {
		b.background.FillColor = color.Transparent
		b.background.Refresh()
	}
	if b.timer != nil {
		b.timer.Stop()
	}
	b.hideTip()
}

func (b *iconTipButton) showTip() {
	if b.popUp != nil || b.tooltip == "" {
		return
	}
	lbl := widget.NewLabel(b.tooltip)
	b.popUp = widget.NewPopUp(lbl, b.win.Canvas())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b)
	pos = pos.Add(fyne.NewPos(0, b.Size().Height+4))
	b.popUp.ShowAtPosition(pos)
}

func (b *iconTipButton) hideTip() {
	if b.popUp != nil {
		b.popUp.Hide()
		b.popUp = nil
	}
}
