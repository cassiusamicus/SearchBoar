package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/internal/regexbuilder"
)

// wizardHelpText explains, in plain language, how to use the pattern types
// that take more than a single literal string -- these are the ones this
// dialog exists to spare the user from having to write regex for.
func wizardHelpText(t regexbuilder.PatternType) string {
	switch t {
	case regexbuilder.AllWords:
		return "Matches a line that contains every one of these words, in any order. Separate words with commas, e.g. cat, dog, bird -- no regex needed."
	case regexbuilder.AnyWord:
		return "Matches a line that contains any one of these words. Separate words with commas, e.g. cat, dog, bird -- no regex needed."
	case regexbuilder.NearWords:
		return "Matches when both words appear on the same line, no more than the given number of words apart, in either order -- no regex needed."
	default:
		return ""
	}
}

// showRegexBuilderDialog reimplements RegexBuilderDialog: pick a pattern
// type, fill in text, optionally toggle case sensitivity, see the generated
// regex (hand-editable), and live-test it against sample text.
func showRegexBuilderDialog(win fyne.Window, title, initialText string, onOK func(pattern string)) {
	labels := make([]string, len(regexbuilder.PatternTypes))
	for i, t := range regexbuilder.PatternTypes {
		labels[i] = t.String()
	}

	textEntry := widget.NewEntry()
	textLabel := widget.NewLabel("Text:")
	word2Entry := widget.NewEntry()
	word2Entry.SetPlaceHolder("second word")
	distanceEntry := widget.NewEntry()
	distanceEntry.SetText("10")
	helpLabel := widget.NewLabel("")
	helpLabel.Wrapping = fyne.TextWrapWord
	helpLabel.TextStyle = fyne.TextStyle{Italic: true}
	caseCheck := widget.NewCheck("Case sensitive", nil)
	regexView := widget.NewMultiLineEntry()
	regexView.SetMinRowsVisible(2)
	testEntry := widget.NewEntry()
	testEntry.SetPlaceHolder("Test text:")
	resultLabel := widget.NewLabel("")

	proximityRow := container.NewBorder(nil, nil, widget.NewLabel("Second word:"), nil, word2Entry)
	distanceRow := container.NewBorder(nil, nil, widget.NewLabel("Max words apart:"), nil, distanceEntry)
	proximityBox := container.NewVBox(proximityRow, distanceRow)
	proximityBox.Hide()

	current := regexbuilder.Builder{Type: regexbuilder.Contains, Distance: 10}

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

		switch {
		case current.Type.IsWordList():
			textLabel.SetText("Words (comma-separated):")
			textEntry.SetPlaceHolder("e.g. cat, dog, bird")
		case current.Type.IsProximity():
			textLabel.SetText("First word:")
			textEntry.SetPlaceHolder("first word")
		default:
			textLabel.SetText("Text:")
			textEntry.SetPlaceHolder("")
		}

		if current.Type.IsProximity() {
			proximityBox.Show()
		} else {
			proximityBox.Hide()
		}

		helpLabel.SetText(wizardHelpText(current.Type))

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

	word2Entry.OnChanged = func(s string) {
		current.Text2 = s
		regenerate()
		runTest()
	}

	distanceEntry.OnChanged = func(s string) {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
			current.Distance = n
		} else {
			current.Distance = 0
		}
		regenerate()
		runTest()
	}

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
		container.NewBorder(nil, nil, textLabel, nil, textEntry),
		proximityBox,
		helpLabel,
		caseCheck,
		widget.NewCard("Generated Regular Expression", "", regexView),
		widget.NewCard("Test Pattern", "", container.NewVBox(testEntry, resultLabel)),
	)

	d := dialog.NewCustomConfirm(title, "OK", "Cancel", form, func(ok bool) {
		if ok {
			onOK(regexView.Text)
		}
	}, win)
	d.Resize(fyne.NewSize(500, 520))
	d.Show()
}
