package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
)

// openConfigFile opens config.ini in the user's default text editor -- kept
// as an escape hatch for anyone who wants raw access (or is troubleshooting
// a config problem), but the Settings dialog is the normal way to change
// anything now; edits there take effect immediately rather than needing a
// restart.
func (a *App) openConfigFile() {
	fsutil.OpenPath(config.DefaultPath())
}

// showSettingsDialog is the gear icon's target: a real settings UI (custom
// "open with" program associations, the search-result cache) instead of
// sending the user straight to a hand-edited text file.
func (a *App) showSettingsDialog() {
	tabs := container.NewAppTabs(
		container.NewTabItem("Custom Programs", a.buildCustomProgramsSettings()),
		container.NewTabItem("Search Cache", a.buildCacheSettings()),
	)

	openFileBtn := widget.NewButton("Open config.ini in text editor...", func() { a.openConfigFile() })

	content := container.NewBorder(nil, openFileBtn, nil, nil, tabs)

	d := dialog.NewCustom("Settings", "Close", content, a.win)
	d.Resize(fyne.NewSize(520, 420))
	d.Show()
}

// buildCustomProgramsSettings lists every saved custom "open with" program
// (see contextmenu.go's showOpenWithDialog, the only other place that wrote
// to CustomPrograms until now) with a way to remove one, plus add a new one
// without going through the Open With flow first.
func (a *App) buildCustomProgramsSettings() fyne.CanvasObject {
	names := customProgramNames(a.cfg.CustomPrograms)

	// Row content is wrapped in tappableBox (a proper widget.Widget via
	// BaseWidget+CreateRenderer) rather than a bare *fyne.Container --
	// widget.List silently renders nothing for rows whose template isn't a
	// real Widget, a bug already hit and fixed once in commontermstab.go.
	var list *widget.List
	list = widget.NewList(
		func() int { return len(names) },
		func() fyne.CanvasObject {
			return newTappableBox(widget.NewLabel(""), layout.NewSpacer(), widget.NewButtonWithIcon("", theme.DeleteIcon(), nil))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*tappableBox)
			lbl := row.box.Objects[0].(*widget.Label)
			btn := row.box.Objects[2].(*widget.Button)
			if id >= len(names) {
				lbl.SetText("")
				return
			}
			name := names[id]
			lbl.SetText(fmt.Sprintf("%s  —  %s", name, a.cfg.CustomPrograms[name]))
			btn.OnTapped = func() {
				delete(a.cfg.CustomPrograms, name)
				a.cfg.Save()
				names = customProgramNames(a.cfg.CustomPrograms)
				list.Refresh()
			}
		},
	)

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Name")
	cmdEntry := widget.NewEntry()
	cmdEntry.SetPlaceHolder("Command (e.g. /usr/bin/gedit)")
	addBtn := widget.NewButton("Add", func() {
		if nameEntry.Text == "" || cmdEntry.Text == "" {
			return
		}
		a.cfg.CustomPrograms[nameEntry.Text] = cmdEntry.Text
		a.cfg.Save()
		names = customProgramNames(a.cfg.CustomPrograms)
		list.Refresh()
		nameEntry.SetText("")
		cmdEntry.SetText("")
	})

	return container.NewBorder(
		widget.NewLabel("Programs available in the right-click \"Open with...\" menu."),
		container.NewVBox(widget.NewSeparator(), nameEntry, cmdEntry, addBtn),
		nil, nil, list,
	)
}

func customProgramNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// buildCacheSettings surfaces the search-result text cache's current size
// and a way to clear it -- the cache has no automatic size cap (see
// internal/cache's package doc: it only ever grows from files the user has
// actually searched, never a background index), so this is the one place a
// user can see it exist and reclaim the disk space.
func (a *App) buildCacheSettings() fyne.CanvasObject {
	desc := widget.NewLabel("Text extracted from files you've searched is cached so repeat searches over the same files skip re-reading/re-extracting them. Nothing is cached until you search for it.")
	desc.Wrapping = fyne.TextWrapWord

	cacheLabel := widget.NewLabel(a.cacheStatsText())
	clearBtn := widget.NewButton("Clear search cache", func() {
		a.cache.Clear()
		cacheLabel.SetText(a.cacheStatsText())
	})

	return container.NewVBox(desc, cacheLabel, clearBtn)
}

func (a *App) cacheStatsText() string {
	rows, bytes, err := a.cache.Stats()
	if err != nil {
		return "Cache: unavailable"
	}
	return fmt.Sprintf("Cache: %d file(s), %s", rows, formatSize(bytes))
}
