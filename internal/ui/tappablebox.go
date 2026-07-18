package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// tappableBox wraps a *fyne.Container so it can report single tap, double
// tap, and secondary tap -- used for the Overview tab's cards, which need
// double-click-to-open and right-click context menus, neither of which
// widget.List exposes per-row. Content is mutated in place via SetObjects
// (rather than replacing the container) so it works with List's
// create-once/update-in-place recycling.
type tappableBox struct {
	widget.BaseWidget

	box *fyne.Container

	OnTapped          func()
	OnDoubleTapped    func()
	OnSecondaryTapped func(*fyne.PointEvent)
}

func newTappableBox(objects ...fyne.CanvasObject) *tappableBox {
	b := &tappableBox{box: container.NewVBox(objects...)}
	b.ExtendBaseWidget(b)
	return b
}

func (b *tappableBox) SetObjects(objects []fyne.CanvasObject) {
	b.box.Objects = objects
	b.box.Refresh()
}

func (b *tappableBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.box)
}

func (b *tappableBox) Tapped(*fyne.PointEvent) {
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

func (b *tappableBox) DoubleTapped(*fyne.PointEvent) {
	if b.OnDoubleTapped != nil {
		b.OnDoubleTapped()
	}
}

func (b *tappableBox) TappedSecondary(e *fyne.PointEvent) {
	if b.OnSecondaryTapped != nil {
		b.OnSecondaryTapped(e)
	}
}
