package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type advancedTab struct {
	app *App

	beforeSpin *intSpinner
	afterSpin  *intSpinner

	minSizeEntry *widget.Entry
	maxSizeEntry *widget.Entry

	excludeEntry *widget.Entry
}

func newAdvancedTab(a *App) *advancedTab {
	return &advancedTab{app: a}
}

func (t *advancedTab) build() fyne.CanvasObject {
	t.beforeSpin = newIntSpinner(0, 10, 2)
	t.afterSpin = newIntSpinner(0, 10, 2)
	contextCard := widget.NewCard("Context Lines", "", container.NewHBox(
		widget.NewLabel("Lines before:"), t.beforeSpin.build(),
		widget.NewLabel("Lines after:"), t.afterSpin.build(),
	))

	t.minSizeEntry = widget.NewEntry()
	t.minSizeEntry.SetText("0")
	t.maxSizeEntry = widget.NewEntry()
	t.maxSizeEntry.SetText("0")
	sizeCard := widget.NewCard("File Size Filter", "", container.NewHBox(
		widget.NewLabel("Min size (KB):"), t.minSizeEntry,
		widget.NewLabel("Max size (KB):"), t.maxSizeEntry,
	))

	t.excludeEntry = widget.NewEntry()
	t.excludeEntry.SetPlaceHolder("e.g., *.pyc,*.o,*.tmp")
	excludeCard := widget.NewCard("Exclude Patterns (glob, comma-separated)", "", t.excludeEntry)

	return container.NewVBox(contextCard, sizeCard, excludeCard)
}

func (t *advancedTab) minSizeBytes() int64 {
	kb, _ := strconv.ParseInt(t.minSizeEntry.Text, 10, 64)
	return kb * 1024
}

func (t *advancedTab) maxSizeBytes() int64 {
	kb, _ := strconv.ParseInt(t.maxSizeEntry.Text, 10, 64)
	return kb * 1024
}

// intSpinner is a minimal +/- numeric stepper; Fyne has no built-in spin
// button widget.
type intSpinner struct {
	min, max, value int
	label           *widget.Label
}

func newIntSpinner(min, max, value int) *intSpinner {
	return &intSpinner{min: min, max: max, value: value}
}

func (s *intSpinner) build() fyne.CanvasObject {
	s.label = widget.NewLabel(strconv.Itoa(s.value))
	dec := widget.NewButton("-", func() { s.set(s.value - 1) })
	inc := widget.NewButton("+", func() { s.set(s.value + 1) })
	return container.NewHBox(dec, s.label, inc)
}

func (s *intSpinner) set(v int) {
	if v < s.min {
		v = s.min
	}
	if v > s.max {
		v = s.max
	}
	s.value = v
	s.label.SetText(fmt.Sprintf("%d", v))
}
