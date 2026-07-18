package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// tappableLabel is a widget.Label that also reports single tap, double
// tap, and secondary (right-click) tap -- used as Table/List cell content
// so rows can open on double-click and show a context menu on right-click,
// which the stock widgets don't expose per-cell.
type tappableLabel struct {
	widget.Label

	OnTapped          func()
	OnDoubleTapped    func()
	OnSecondaryTapped func(*fyne.PointEvent)
}

func newTappableLabel() *tappableLabel {
	l := &tappableLabel{}
	l.ExtendBaseWidget(l)
	return l
}

func (l *tappableLabel) Tapped(*fyne.PointEvent) {
	if l.OnTapped != nil {
		l.OnTapped()
	}
}

func (l *tappableLabel) DoubleTapped(*fyne.PointEvent) {
	if l.OnDoubleTapped != nil {
		l.OnDoubleTapped()
	}
}

func (l *tappableLabel) TappedSecondary(e *fyne.PointEvent) {
	if l.OnSecondaryTapped != nil {
		l.OnSecondaryTapped(e)
	}
}
