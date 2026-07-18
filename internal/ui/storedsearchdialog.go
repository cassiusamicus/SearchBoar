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
// save the current Basic-tab search, load a saved one back into the Basic
// tab, or delete one.
func (a *App) showStoredSearchesDialog() {
	searches := a.ssStore.List()
	selected := -1

	table := widget.NewTable(
		func() (int, int) { return len(searches), 4 },
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
			case 3:
				l.SetText(s.Directory)
			}
		},
	)
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		headers := []string{"Name", "File Pattern", "Content Pattern", "Directory"}
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
		a.basic.fileEntry.SetText(s.FilePattern)
		a.basic.contentCombo.SetText(s.ContentPattern)
		a.basic.dirCombo.SetText(s.Directory)
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
	win.Resize(fyne.NewSize(600, 400))
	win.Show()
}

func (a *App) promptSaveCurrentSearch(onSaved func()) {
	dialog.ShowEntryDialog("Save Current Search", "Name:", func(name string) {
		if name == "" {
			return
		}
		err := a.ssStore.Add(name, config.StoredSearch{
			FilePattern:    a.basic.fileEntry.Text,
			ContentPattern: a.basic.contentCombo.Text,
			Directory:      a.basic.dirCombo.Text,
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
