package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
)

// fileContextMenu builds the right-click menu shared by the Details file
// table and the Overview card list: Open in default program, Open with...,
// Show in file manager, Copy path, Add/Remove Favorite, Delete file.
func (a *App) fileContextMenu(rec fileRecord, onChanged func()) *fyne.Menu {
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Open in default program", func() {
			if err := fsutil.OpenPath(rec.Path); err != nil {
				a.setStatus("Failed to open: " + err.Error())
			}
		}),
		fyne.NewMenuItem("Open with...", func() { a.showOpenWithDialog(rec.Path) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Show in file manager", func() {
			if err := fsutil.ShowInFileManager(rec.Path); err != nil {
				a.setStatus("Failed to open file manager: " + err.Error())
			}
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Copy path", func() {
			a.win.Clipboard().SetContent(rec.Path)
			a.setStatus("Path copied to clipboard")
		}),
		fyne.NewMenuItemSeparator(),
	}

	if a.favStore.Contains(rec.Path) {
		items = append(items, fyne.NewMenuItem("Remove from Favorites", func() {
			a.favStore.Remove(rec.Path)
			a.favStore.Save()
			a.favTab.refresh()
			if onChanged != nil {
				onChanged()
			}
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Add to Favorites", func() {
			a.addFavorite(rec)
			if onChanged != nil {
				onChanged()
			}
		}))
	}

	items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Delete file", func() {
		dialog.ShowConfirm("Delete File?",
			fmt.Sprintf("Are you sure you want to delete: %s. This action cannot be undone.", rec.Path),
			func(ok bool) {
				if !ok {
					return
				}
				if err := fsutil.DeleteFile(rec.Path); err != nil {
					a.setStatus("Failed to delete: " + err.Error())
					return
				}
				a.setStatus("Deleted " + rec.Path)
				if onChanged != nil {
					onChanged()
				}
			}, a.win)
	}))

	return fyne.NewMenu("", items...)
}

func (a *App) showOpenWithDialog(path string) {
	candidates := fsutil.CandidatePrograms()
	for name, cmd := range a.cfg.CustomPrograms {
		candidates = append(candidates, fsutil.Program{Label: name + " (custom)", Command: cmd})
	}

	list := widget.NewList(
		func() int { return len(candidates) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(candidates[i].Label) },
	)

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Name")
	cmdEntry := widget.NewEntry()
	cmdEntry.SetPlaceHolder("Command")

	selected := -1
	list.OnSelected = func(id widget.ListItemID) { selected = id }

	addBtn := widget.NewButton("Add Custom Program", func() {
		if nameEntry.Text == "" || cmdEntry.Text == "" {
			return
		}
		a.cfg.CustomPrograms[nameEntry.Text] = cmdEntry.Text
		a.cfg.Save()
		candidates = append(candidates, fsutil.Program{Label: nameEntry.Text + " (custom)", Command: cmdEntry.Text})
		list.Refresh()
		nameEntry.SetText("")
		cmdEntry.SetText("")
	})

	content := container.NewBorder(nil,
		container.NewVBox(widget.NewSeparator(), nameEntry, cmdEntry, addBtn),
		nil, nil, list)

	d := dialog.NewCustomConfirm("Open with...", "Open", "Cancel", content, func(ok bool) {
		if !ok || selected < 0 || selected >= len(candidates) {
			return
		}
		if err := fsutil.OpenWith(candidates[selected].Command, path); err != nil {
			a.setStatus("Failed to open: " + err.Error())
		}
	}, a.win)
	d.Resize(fyne.NewSize(420, 420))
	d.Show()
}

func (a *App) addFavorite(rec fileRecord) {
	searchTerm := ""
	if a.basic.contentEnabled.Checked {
		searchTerm = a.basic.contentCombo.Text
	}
	err := a.favStore.Add(config.FavoriteRecord{
		Filepath:   rec.Path,
		Filename:   rec.Name,
		SearchTerm: searchTerm,
		Modified:   rec.ModifiedStr,
		SizeBytes:  rec.Size,
		SizeHuman:  rec.SizeHuman,
		Category:   "Uncategorized",
	})
	if err != nil {
		a.setStatus(err.Error())
		return
	}
	a.favStore.Save()
	a.favTab.refresh()
	a.setStatus("Added to favorites")
}

// fileRecord is the minimal set of fields the context menu and favorites
// need about a result row, shared by the Details table and Overview cards.
type fileRecord struct {
	Path        string
	Name        string
	ModifiedStr string
	Size        int64
	SizeHuman   string
}
