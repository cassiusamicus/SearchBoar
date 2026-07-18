package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// showStoredSearchesDialog reimplements the "Stored Searches" dialog:
// save the current Search Builder pattern, load a saved one back into it,
// or delete one. Stored searches capture the filename/content pattern
// only -- since where to search is now a persistent location-picker state
// rather than a single directory, there's no per-search directory to
// save/restore.
func (a *App) showStoredSearchesDialog() {
	searches := a.ssStore.List()
	selected := -1

	table := widget.NewTable(
		func() (int, int) { return len(searches), 3 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(searches) {
				l.SetText("")
				return
			}
			s := searches[id.Row]
			switch id.Col {
			case 0:
				l.SetText(s.Name)
			case 1:
				l.SetText(s.FilePattern)
			case 2:
				l.SetText(s.ContentPattern)
			}
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		headers := []string{"Name", "File Pattern", "Content Pattern"}
		o.(*widget.Label).SetText(headers[id.Col])
	}
	table.OnSelected = func(id widget.TableCellID) { selected = id.Row }

	var win dialog.Dialog

	saveBtn := widget.NewButton("Save Current Search", func() {
		a.promptSaveCurrentSearch(func() {
			searches = a.ssStore.List()
			table.Refresh()
		})
	})
	loadBtn := widget.NewButton("Load Selected", func() {
		if selected < 0 || selected >= len(searches) {
			return
		}
		s := searches[selected]
		a.builder.fileEntry.SetText(s.FilePattern)
		a.builder.contentCombo.SetText(s.ContentPattern)
		if win != nil {
			win.Hide()
		}
	})
	deleteBtn := widget.NewButton("Delete Selected", func() {
		if selected < 0 || selected >= len(searches) {
			return
		}
		name := searches[selected].Name
		dialog.ShowConfirm("Delete Favorite Search", fmt.Sprintf("Are you sure you want to delete %q?", name), func(ok bool) {
			if !ok {
				return
			}
			a.ssStore.Delete(name)
			a.ssStore.Save()
			searches = a.ssStore.List()
			selected = -1
			table.Refresh()
		}, a.win)
	})

	content := container.NewBorder(
		container.NewHBox(saveBtn, loadBtn, deleteBtn), nil, nil, nil, table,
	)

	win = dialog.NewCustom("Stored Searches", "Close", content, a.win)
	win.Resize(fyne.NewSize(500, 400))
	win.Show()
}

func (a *App) promptSaveCurrentSearch(onSaved func()) {
	dialog.ShowEntryDialog("Save Current Search", "Name:", func(name string) {
		if name == "" {
			return
		}
		err := a.ssStore.Add(name, config.StoredSearch{
			FilePattern:    a.builder.fileEntry.Text,
			ContentPattern: a.builder.contentCombo.Text,
		})
		if err != nil {
			a.setStatus(err.Error())
			return
		}
		a.ssStore.Save()
		if onSaved != nil {
			onSaved()
		}
	}, a.win)
}
