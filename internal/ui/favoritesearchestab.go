package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

// favoriteSearchesTab is the "Favorite Searches" tab: save/load/delete
// named filename/content patterns (promoted from a modal dialog to a full
// tab, alongside Favorite Results). Loading a saved search fills in the
// Start tab's search fields and switches to it.
type favoriteSearchesTab struct {
	app *App

	table    *widget.Table
	searches []config.StoredSearch
	selected int
}

func newFavoriteSearchesTab(a *App) *favoriteSearchesTab {
	return &favoriteSearchesTab{app: a, selected: -1}
}

func (t *favoriteSearchesTab) build() fyne.CanvasObject {
	t.searches = t.app.ssStore.List()

	t.table = widget.NewTable(
		func() (int, int) { return len(t.searches), 3 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(t.searches) {
				l.SetText("")
				return
			}
			s := t.searches[id.Row]
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
	t.table.ShowHeaderRow = true
	t.table.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	t.table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		headers := []string{"Name", "File Pattern", "Content Pattern"}
		o.(*widget.Label).SetText(headers[id.Col])
	}
	t.table.OnSelected = func(id widget.TableCellID) { t.selected = id.Row }
	t.table.SetColumnWidth(0, 200)
	t.table.SetColumnWidth(1, 250)
	t.table.SetColumnWidth(2, 250)

	saveBtn := widget.NewButton("Save Current Search", func() { t.promptSaveCurrentSearch() })
	loadBtn := widget.NewButton("Load Selected", func() { t.loadSelected() })
	deleteBtn := widget.NewButton("Delete Selected", func() { t.deleteSelected() })

	return container.NewBorder(
		container.NewHBox(saveBtn, loadBtn, deleteBtn), nil, nil, nil, t.table,
	)
}

func (t *favoriteSearchesTab) refresh() {
	t.searches = t.app.ssStore.List()
	if t.table != nil {
		t.table.Refresh()
	}
}

func (t *favoriteSearchesTab) loadSelected() {
	if t.selected < 0 || t.selected >= len(t.searches) {
		t.app.setStatus("Select a stored search to load")
		return
	}
	s := t.searches[t.selected]
	t.app.start.fileEntry.SetText(s.FilePattern)
	t.app.start.contentCombo.SetText(s.ContentPattern)
	t.app.tabs.SelectIndex(tabIndexStart)
}

func (t *favoriteSearchesTab) deleteSelected() {
	if t.selected < 0 || t.selected >= len(t.searches) {
		t.app.setStatus("Select a stored search to delete")
		return
	}
	name := t.searches[t.selected].Name
	dialog.ShowConfirm("Delete Favorite Search", fmt.Sprintf("Are you sure you want to delete %q?", name), func(ok bool) {
		if !ok {
			return
		}
		t.app.ssStore.Delete(name)
		t.app.ssStore.Save()
		t.selected = -1
		t.refresh()
		t.app.start.refreshSavedSearches()
	}, t.app.win)
}

func (t *favoriteSearchesTab) promptSaveCurrentSearch() {
	dialog.ShowEntryDialog("Save Current Search", "Name:", func(name string) {
		if name == "" {
			return
		}
		err := t.app.ssStore.Add(name, config.StoredSearch{
			FilePattern:    t.app.start.fileEntry.Text,
			ContentPattern: t.app.start.contentCombo.Text,
		})
		if err != nil {
			t.app.setStatus(err.Error())
			return
		}
		t.app.ssStore.Save()
		t.refresh()
		t.app.start.refreshSavedSearches()
	}, t.app.win)
}
