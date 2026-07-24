package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/cassiusamicus/SearchBoar/internal/fsutil"
)

// checkDependenciesOnStartup mirrors the original app's automatic
// dependency check on every launch: if ripgrep or pdftotext are missing,
// show a copyable per-distro install command. Unlike the original, there
// is no "missing pypdf" case to report -- PDF/DOCX extraction always work
// via statically linked pure-Go fallbacks.
func (a *App) checkDependenciesOnStartup() {
	deps := fsutil.CheckOptionalDependencies()
	var missing []string
	for _, d := range deps {
		if !d.Present {
			missing = append(missing, d.Key)
		}
	}
	if len(missing) == 0 {
		return
	}
	runOnUI(func() { a.showDependencyDialog(missing) })
}

func (a *App) showDependencyDialog(missingKeys []string) {
	command, distroLabel := fsutil.BuildInstallCommand(missingKeys)

	deps := fsutil.CheckOptionalDependencies()
	desc := "SearchBoar works without these, but installing them provides:\n"
	for _, d := range deps {
		if !d.Present {
			desc += "\n• " + d.Label + ": " + d.Description
		}
	}
	desc += "\n\nDetected system: " + distroLabel
	desc += "\n\nCopy and paste this command to install the missing packages:"

	cmdEntry := widget.NewMultiLineEntry()
	cmdEntry.SetText(command)
	cmdEntry.Wrapping = fyne.TextWrapOff

	copyBtn := widget.NewButton("Copy to clipboard", func() {
		a.win.Clipboard().SetContent(command)
	})

	content := container.NewVBox(
		widget.NewLabel(desc),
		cmdEntry,
		copyBtn,
	)

	d := dialog.NewCustom("Optional Dependencies Missing", "OK", content, a.win)
	d.Resize(fyne.NewSize(520, 320))
	d.Show()
}
