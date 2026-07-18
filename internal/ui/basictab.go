package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

type basicTab struct {
	app *App

	fileTypeChecks map[string]*widget.Check
	updatingTypes  bool

	fileEntry      *widget.Entry
	contentEnabled *widget.Check
	contentCombo   *widget.SelectEntry
	dirCombo       *widget.SelectEntry

	recursiveCheck *widget.Check
	caseCheck      *widget.Check
	hiddenCheck    *widget.Check
}

func newBasicTab(a *App) *basicTab {
	return &basicTab{app: a, fileTypeChecks: map[string]*widget.Check{}}
}

func (b *basicTab) build() fyne.CanvasObject {
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

	b.dirCombo = widget.NewSelectEntry(b.app.cfg.Recent.Paths)
	if len(b.app.cfg.Recent.Paths) > 0 {
		b.dirCombo.SetText(b.app.cfg.Recent.Paths[0])
	} else if home, err := homeDir(); err == nil {
		b.dirCombo.SetText(home)
	}
	b.dirCombo.OnSubmitted = func(string) { b.app.startSearch() }
	browseBtn := widget.NewButton("Browse...", func() {
		d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			b.dirCombo.SetText(uri.Path())
		}, b.app.win)
		d.Show()
	})
	lookInRow := container.NewBorder(nil, nil, widget.NewLabel("Look in:"), browseBtn, b.dirCombo)

	b.recursiveCheck = widget.NewCheck("Search in subdirectories", nil)
	b.recursiveCheck.SetChecked(true)
	b.caseCheck = widget.NewCheck("Case sensitive", nil)
	b.hiddenCheck = widget.NewCheck("Include hidden files", nil)
	optionsRow := container.NewHBox(b.recursiveCheck, b.caseCheck, b.hiddenCheck)

	help := widget.NewAccordion(widget.NewAccordionItem("Search Help", searchHelpContent()))

	return container.NewVBox(
		widget.NewLabel("File Types:"), typeRow,
		filesRow,
		containingRow,
		lookInRow,
		widget.NewCard("Options", "", optionsRow),
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
func (b *basicTab) onFileTypeToggled(changed string) {
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
