package ui

import (
	"context"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/fsutil"
	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
)

var netFileTypePatterns = []struct{ label, pattern string }{
	{"All", "*"},
	{"Images", "*.{jpg,jpeg,png,gif,bmp,webp,tiff,svg}"},
	{"PDFs", "*.pdf"},
	{"Text", "*.{txt,md,org,rst,log}"},
	{"Videos", "*.{mp4,avi,mkv,mov,wmv,flv,webm,m4v,mpg,mpeg}"},
	{"Audio", "*.{mp3,wav,flac,m4a,aac,ogg,wma,opus}"},
	{"Docs", "*.{doc,docx,odt,pdf,rtf}"},
}

type networkTab struct {
	app *App

	containsEntry *widget.Entry
	patternEntry  *widget.Entry

	picker *drivePicker

	cidrEntry *widget.Entry
	userEntry *widget.Entry
	passEntry *widget.Entry

	localCheck, smbCheck, nfsCheck *widget.Check
	searchBtn, stopBtn             *widget.Button

	table   *widget.Table
	results []netsearch.Result
	order   []int
	sortCol int
	sortAsc bool

	logView *widget.List
	logs    []netsearch.LogLine

	progressBar *widget.ProgressBar
	statusLabel *widget.Label

	engine       *netsearch.Engine
	cancelSearch context.CancelFunc
}

func newNetworkTab(a *App) *networkTab {
	return &networkTab{app: a, engine: netsearch.NewEngine(), picker: newDrivePicker(), sortAsc: true}
}

func (t *networkTab) build() fyne.CanvasObject {
	patternBuilder := t.buildPatternBuilder()
	drivePicker := widget.NewCard("Storage Locations", "", t.picker.build())
	settingsActions := container.NewVBox(t.buildNetworkSettings(), t.buildActions())

	top := container.NewGridWithColumns(3, patternBuilder, drivePicker, settingsActions)

	t.progressBar = widget.NewProgressBar()
	t.progressBar.Hide()
	t.statusLabel = widget.NewLabel("Ready")

	resultsCard := widget.NewCard("Search Results", "", t.buildResultsTable())
	logCard := widget.NewCard("Search Log", "", t.buildLogView())

	middle := container.NewVSplit(resultsCard, logCard)
	middle.Offset = 0.7

	return container.NewBorder(
		container.NewVBox(top, t.progressBar, t.statusLabel),
		nil, nil, nil,
		middle,
	)
}

func (t *networkTab) buildPatternBuilder() fyne.CanvasObject {
	labels := make([]string, len(netFileTypePatterns))
	for i, p := range netFileTypePatterns {
		labels[i] = p.label
	}
	containsEntry := widget.NewEntry()
	containsEntry.SetPlaceHolder("vacation, report...")
	t.containsEntry = containsEntry

	t.patternEntry = widget.NewEntry()
	t.patternEntry.SetText("*")

	updatePattern := func(basePattern string) {
		contains := t.containsEntry.Text
		if basePattern == "*" {
			if contains != "" {
				t.patternEntry.SetText("*" + contains + "*")
			} else {
				t.patternEntry.SetText("*")
			}
			return
		}
		// Splice contains-text between the leading "*" and the extension
		// group, e.g. "*.{jpg,png}" + "vacation" -> "*vacation*.{jpg,png}".
		if contains != "" {
			t.patternEntry.SetText("*" + contains + strings.TrimPrefix(basePattern, "*"))
		} else {
			t.patternEntry.SetText(basePattern)
		}
	}

	radio := widget.NewRadioGroup(labels, func(selected string) {
		for _, p := range netFileTypePatterns {
			if p.label == selected {
				updatePattern(p.pattern)
			}
		}
	})
	radio.SetSelected("All")
	// Vertical (not Horizontal) so this card doesn't demand an oversized
	// natural width -- RadioGroup's horizontal MinSize is width-per-item
	// times item count, which the enclosing 3-column grid then multiplies
	// by 3, making the whole window unable to shrink below ~1900px.

	t.containsEntry.OnChanged = func(string) {
		sel := radio.Selected
		for _, p := range netFileTypePatterns {
			if p.label == sel {
				updatePattern(p.pattern)
			}
		}
	}

	return widget.NewCard("Pattern Builder", "", container.NewVBox(
		radio,
		container.NewBorder(nil, nil, widget.NewLabel("Contains:"), nil, t.containsEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Pattern:"), nil, t.patternEntry),
	))
}

func (t *networkTab) buildNetworkSettings() fyne.CanvasObject {
	t.cidrEntry = widget.NewEntry()
	t.cidrEntry.SetPlaceHolder("auto or 192.168.1.0/24")
	t.userEntry = widget.NewEntry()
	t.passEntry = widget.NewPasswordEntry()

	return widget.NewCard("Network Settings", "", container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Range:"), nil, t.cidrEntry),
		widget.NewLabel("SMB (optional):"),
		container.NewBorder(nil, nil, widget.NewLabel("User:"), nil, t.userEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Pass:"), nil, t.passEntry),
	))
}

func (t *networkTab) buildActions() fyne.CanvasObject {
	t.localCheck = widget.NewCheck("Local", nil)
	t.localCheck.SetChecked(true)
	t.smbCheck = widget.NewCheck("SMB", nil)
	t.smbCheck.SetChecked(true)
	t.nfsCheck = widget.NewCheck("NFS", nil)
	t.nfsCheck.SetChecked(true)

	t.searchBtn = widget.NewButton("Search", func() { t.startSearch() })
	t.stopBtn = widget.NewButton("Stop", func() { t.stopSearch() })
	t.stopBtn.Disable()
	clearBtn := widget.NewButton("Clear Results", func() { t.clearResults() })

	return widget.NewCard("Actions", "", container.NewVBox(
		container.NewHBox(t.localCheck, t.smbCheck, t.nfsCheck),
		widget.NewSeparator(),
		container.NewHBox(t.searchBtn, t.stopBtn, clearBtn),
	))
}

func (t *networkTab) buildResultsTable() fyne.CanvasObject {
	t.table = widget.NewTable(
		func() (int, int) { return len(t.order), 3 },
		func() fyne.CanvasObject { return newTappableLabel() },
		func(id widget.TableCellID, o fyne.CanvasObject) { t.updateCell(id, o.(*tappableLabel)) },
	)
	t.table.ShowHeaderRow = true
	t.table.CreateHeader = func() fyne.CanvasObject { return widget.NewButton("", nil) }
	t.table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		b := o.(*widget.Button)
		headers := []string{"Network Path", "Modified", "Size"}
		b.SetText(headers[id.Col])
		col := id.Col
		b.OnTapped = func() { t.sortBy(col) }
	}
	t.table.SetColumnWidth(0, 420)
	t.table.SetColumnWidth(1, 150)
	t.table.SetColumnWidth(2, 100)
	return t.table
}

func (t *networkTab) updateCell(id widget.TableCellID, l *tappableLabel) {
	if id.Row >= len(t.order) {
		l.SetText("")
		return
	}
	r := t.results[t.order[id.Row]]
	switch id.Col {
	case 0:
		l.SetText(r.NetworkPath)
	case 1:
		l.SetText(r.Modified)
	case 2:
		l.SetText(formatSize(r.Size))
	}
	l.OnDoubleTapped = func() {
		if err := fsutil.OpenPath(r.LocalPath); err != nil {
			t.setStatus("Failed to open: " + err.Error())
		}
	}
	l.OnSecondaryTapped = func(e *fyne.PointEvent) {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Open in default program", func() {
				if err := fsutil.OpenPath(r.LocalPath); err != nil {
					t.setStatus("Failed to open: " + err.Error())
				}
			}),
			fyne.NewMenuItem("Show in file manager", func() {
				if err := fsutil.ShowInFileManager(r.LocalPath); err != nil {
					t.setStatus("Failed to open file manager: " + err.Error())
				}
			}),
			fyne.NewMenuItem("Copy path", func() {
				t.app.win.Clipboard().SetContent(r.NetworkPath)
				t.setStatus("Path copied to clipboard")
			}),
		)
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

func (t *networkTab) sortBy(col int) {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
		t.sortAsc = true
	}
	t.resort()
}

func (t *networkTab) resort() {
	order := make([]int, len(t.results))
	for i := range order {
		order[i] = i
	}
	results := t.results
	sort.SliceStable(order, func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		var lt bool
		switch t.sortCol {
		case 1:
			lt = a.Modified < b.Modified
		case 2:
			lt = a.Size < b.Size
		default:
			lt = a.NetworkPath < b.NetworkPath
		}
		if !t.sortAsc {
			return !lt
		}
		return lt
	})
	t.order = order
	t.table.Refresh()
}

func (t *networkTab) buildLogView() fyne.CanvasObject {
	t.logView = widget.NewList(
		func() int { return len(t.logs) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			line := t.logs[id]
			o.(*widget.Label).SetText("[" + line.Level + "] " + line.Message)
		},
	)
	return t.logView
}

func (t *networkTab) setStatus(msg string) {
	t.statusLabel.SetText(msg)
}

func (t *networkTab) clearResults() {
	if t.cancelSearch != nil {
		t.stopSearch()
	}
	t.results = nil
	t.order = nil
	t.logs = nil
	t.table.Refresh()
	t.logView.Refresh()
	t.engine.Mounts.UnmountAll(context.Background())
	t.setStatus("Ready")
}
