package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/regexbuilder"
)

// showRegexBuilderDialog reimplements RegexBuilderDialog: pick a pattern
// type, fill in text, optionally toggle case sensitivity, see the generated
// regex (hand-editable), and live-test it against sample text.
func showRegexBuilderDialog(win fyne.Window, title, initialText string, onOK func(pattern string)) {
	labels := make([]string, len(regexbuilder.PatternTypes))
	for i, t := range regexbuilder.PatternTypes {
		labels[i] = t.String()
	}

	textEntry := widget.NewEntry()
	caseCheck := widget.NewCheck("Case sensitive", nil)
	regexView := widget.NewMultiLineEntry()
	regexView.SetMinRowsVisible(2)
	testEntry := widget.NewEntry()
	testEntry.SetPlaceHolder("Test text:")
	resultLabel := widget.NewLabel("")

	current := regexbuilder.Builder{Type: regexbuilder.Contains}

	regenerate := func() {
		pattern, err := current.Generate()
		if err != nil {
			regexView.SetText("")
			return
		}
		regexView.SetText(pattern)
	}

	runTest := func() {
		pattern := regexView.Text
		b := regexbuilder.Builder{Type: regexbuilder.CustomRegex, Text: pattern}
		n, err := b.Test(testEntry.Text)
		switch {
		case err != nil:
			resultLabel.Text = "✗ Invalid regex: " + err.Error()
			resultLabel.Importance = widget.DangerImportance
		case n == 0:
			resultLabel.Text = "✗ No match"
			resultLabel.Importance = widget.DangerImportance
		default:
			resultLabel.Text = fmt.Sprintf("✓ Match found (%d occurrence(s))", n)
			resultLabel.Importance = widget.SuccessImportance
		}
		resultLabel.Refresh()
	}

	typeSelect := widget.NewSelect(labels, func(label string) {
		for _, t := range regexbuilder.PatternTypes {
			if t.String() == label {
				current.Type = t
				break
			}
		}
		textEntry.Enable()
		if !current.Type.NeedsText() {
			textEntry.Disable()
		}
		regenerate()
		runTest()
	})
	typeSelect.SetSelected(regexbuilder.Contains.String())

	textEntry.SetText(initialText)
	textEntry.OnChanged = func(s string) {
		current.Text = s
		regenerate()
		runTest()
	}
	current.Text = initialText

	caseCheck.OnChanged = func(v bool) {
		current.CaseSensitive = v
		regenerate()
		runTest()
	}

	regexView.OnChanged = func(string) { runTest() }
	testEntry.OnChanged = func(string) { runTest() }

	regenerate()

	form := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Pattern Type:"), nil, typeSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Text:"), nil, textEntry),
		caseCheck,
		widget.NewCard("Generated Regular Expression", "", regexView),
		widget.NewCard("Test Pattern", "", container.NewVBox(testEntry, resultLabel)),
	)

	d := dialog.NewCustomConfirm(title, "OK", "Cancel", form, func(ok bool) {
		if ok {
			onOK(regexView.Text)
		}
	}, win)
	d.Resize(fyne.NewSize(500, 460))
	d.Show()
}
