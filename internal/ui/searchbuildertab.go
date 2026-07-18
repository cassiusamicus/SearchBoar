package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// fileTypeExtRegex mirrors the original app's file-type checkbox -> regex
// fragment mapping.
var fileTypeExtRegex = map[string]string{
	"MD":   `\.md`,
	"ORG":  `\.org`,
	"TXT":  `\.txt`,
	"HTML": `\.(html|htm)`,
	"PDF":  `\.pdf`,
	"DOCX": `\.docx`,
}

var fileTypeOrder = []string{"ALL", "MD", "ORG", "TXT", "HTML", "PDF", "DOCX"}

// searchBuilderTab is the "Search Builder" tab: what to search for (regex
// filename/content patterns, shared by local and network search since both
// now run through the same internal/search engine), and the file-type
// quick filters, context/size/exclude settings that go with it.
type searchBuilderTab struct {
	app *App

	fileTypeChecks map[string]*widget.Check
	updatingTypes  bool

	fileEntry      *widget.Entry
	contentEnabled *widget.Check
	contentCombo   *widget.SelectEntry

	recursiveCheck *widget.Check
	caseCheck      *widget.Check
	hiddenCheck    *widget.Check

	beforeSpin *intSpinner
	afterSpin  *intSpinner

	minSizeEntry *widget.Entry
	maxSizeEntry *widget.Entry
	excludeEntry *widget.Entry
}

func newSearchBuilderTab(a *App) *searchBuilderTab {
	return &searchBuilderTab{app: a, fileTypeChecks: map[string]*widget.Check{}}
}

func (b *searchBuilderTab) build() fyne.CanvasObject {
	// fileEntry must exist before any checkbox fires onFileTypeToggled
	// (which writes into it), so create it before wiring the checkboxes.
	b.fileEntry = widget.NewEntry()
	b.fileEntry.SetText(".*")
	b.fileEntry.SetPlaceHolder(`e.g., *.py or .*(\.c|\.h)$`)

	typeRow := container.NewHBox()
	for _, name := range fileTypeOrder {
		name := name
		chk := widget.NewCheck(name, func(bool) { b.onFileTypeToggled(name) })
		b.fileTypeChecks[name] = chk
		typeRow.Add(chk)
	}
	b.fileTypeChecks["ALL"].SetChecked(true)

	fileWizardBtn := widget.NewButton("Expr. Wizard", func() {
		showRegexBuilderDialog(b.app.win, "File Name Pattern Builder", b.fileEntry.Text, func(pattern string) {
			b.fileEntry.SetText(pattern)
		})
	})
	filesRow := container.NewBorder(nil, nil, widget.NewLabel("Files:"), fileWizardBtn, b.fileEntry)

	b.contentEnabled = widget.NewCheck("", nil)
	b.contentEnabled.SetChecked(true)
	b.contentCombo = widget.NewSelectEntry(b.app.cfg.Recent.ContentPatterns)
	b.contentCombo.SetPlaceHolder("Text or regex to search for in files")
	b.contentCombo.OnSubmitted = func(string) { b.app.startSearch() }
	contentWizardBtn := widget.NewButton("Expr. Wizard", func() {
		showRegexBuilderDialog(b.app.win, "Content Pattern Builder", b.contentCombo.Text, func(pattern string) {
			b.contentCombo.SetText(pattern)
		})
	})
	containingRow := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewLabel("Containing:"), b.contentEnabled), contentWizardBtn, b.contentCombo)

	b.recursiveCheck = widget.NewCheck("Search in subdirectories", nil)
	b.recursiveCheck.SetChecked(true)
	b.caseCheck = widget.NewCheck("Case sensitive", nil)
	b.hiddenCheck = widget.NewCheck("Include hidden files", nil)
	optionsRow := container.NewHBox(b.recursiveCheck, b.caseCheck, b.hiddenCheck)

	b.beforeSpin = newIntSpinner(0, 10, 2)
	b.afterSpin = newIntSpinner(0, 10, 2)
	contextCard := widget.NewCard("Context Lines", "", container.NewHBox(
		widget.NewLabel("Lines before:"), b.beforeSpin.build(),
		widget.NewLabel("Lines after:"), b.afterSpin.build(),
	))

	b.minSizeEntry = widget.NewEntry()
	b.minSizeEntry.SetText("0")
	b.maxSizeEntry = widget.NewEntry()
	b.maxSizeEntry.SetText("0")
	sizeCard := widget.NewCard("File Size Filter", "", container.NewHBox(
		widget.NewLabel("Min size (KB):"), b.minSizeEntry,
		widget.NewLabel("Max size (KB):"), b.maxSizeEntry,
	))

	b.excludeEntry = widget.NewEntry()
	b.excludeEntry.SetPlaceHolder("e.g., *.pyc,*.o,*.tmp")
	excludeCard := widget.NewCard("Exclude Patterns (glob, comma-separated)", "", b.excludeEntry)

	help := widget.NewAccordion(widget.NewAccordionItem("Search Help", searchHelpContent()))

	return container.NewVBox(
		widget.NewLabel("File Types:"), typeRow,
		filesRow,
		containingRow,
		widget.NewCard("Options", "", optionsRow),
		contextCard,
		sizeCard,
		excludeCard,
		help,
	)
}

func searchHelpContent() fyne.CanvasObject {
	common := widget.NewLabel(
		"Common Searches\n" +
			"• epicurus → finds this word anywhere\n" +
			"• fat and sleek → exact phrase (words together)\n" +
			"• Case sensitive → matches capitalization exactly")
	recipes := widget.NewLabel(
		"Regex Recipes (optional)\n" +
			"• fear|death → either word\n" +
			"• (?s)(?=.*fat)(?=.*sleek) → both words anywhere in the file\n" +
			"• fat.*sleek → words in this order (same line)\n" +
			`• \bfear\b → whole word only` + "\n" +
			"• fear → partial match (inside other words too)\n" +
			"• ^Epicurus → line begins with\n" +
			"• pain$ → line ends with")
	return container.NewHBox(common, widget.NewSeparator(), recipes)
}

// onFileTypeToggled reimplements the original app's checkbox interaction:
// checking ALL clears the others; checking a specific type unchecks ALL and
// combines every checked type into one filename regex.
func (b *searchBuilderTab) onFileTypeToggled(changed string) {
	if b.updatingTypes {
		return
	}
	b.updatingTypes = true
	defer func() { b.updatingTypes = false }()

	if changed == "ALL" {
		if b.fileTypeChecks["ALL"].Checked {
			for _, name := range fileTypeOrder[1:] {
				b.fileTypeChecks[name].SetChecked(false)
			}
			b.fileEntry.SetText(".*")
		}
		return
	}

	if b.fileTypeChecks[changed].Checked {
		b.fileTypeChecks["ALL"].SetChecked(false)
	}

	var parts []string
	for _, name := range fileTypeOrder[1:] {
		if b.fileTypeChecks[name].Checked {
			parts = append(parts, fileTypeExtRegex[name])
		}
	}
	if len(parts) == 0 {
		b.fileTypeChecks["ALL"].SetChecked(true)
		b.fileEntry.SetText(".*")
		return
	}
	b.fileEntry.SetText(".*(" + strings.Join(parts, "|") + ")$")
}

func (b *searchBuilderTab) minSizeBytes() int64 {
	kb, _ := strconv.ParseInt(b.minSizeEntry.Text, 10, 64)
	return kb * 1024
}

func (b *searchBuilderTab) maxSizeBytes() int64 {
	kb, _ := strconv.ParseInt(b.maxSizeEntry.Text, 10, 64)
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
