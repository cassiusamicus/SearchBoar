package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

type detailsTab struct {
	app *App

	sortCol int // 0=name, 1=modified, 2=size
	sortAsc bool
	order   []int // display row -> app.results index
	selRow  int   // -1 if none

	table  *widget.Table
	viewer *widget.TextGrid
}

func newDetailsTab(a *App) *detailsTab {
	return &detailsTab{app: a, sortCol: 0, sortAsc: true, selRow: -1}
}

func (t *detailsTab) build() fyne.CanvasObject {
	t.table = widget.NewTable(
		func() (int, int) { return len(t.order), 3 },
		func() fyne.CanvasObject { return newTappableLabel() },
		func(id widget.TableCellID, o fyne.CanvasObject) { t.updateCell(id, o.(*tappableLabel)) },
	)
	t.table.ShowHeaderRow = true
	t.table.CreateHeader = func() fyne.CanvasObject { return widget.NewButton("", nil) }
	t.table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		b := o.(*widget.Button)
		switch id.Col {
		case 0:
			b.SetText("File")
		case 1:
			b.SetText("Modified")
		case 2:
			b.SetText("Size")
		}
		col := id.Col
		b.OnTapped = func() { t.sortBy(col) }
	}
	t.table.SetColumnWidth(0, 300)
	t.table.SetColumnWidth(1, 150)
	t.table.SetColumnWidth(2, 100)

	t.viewer = widget.NewTextGrid()
	t.viewer.ShowLineNumbers = false

	split := container.NewHSplit(
		container.NewBorder(widget.NewLabel("Files Found"), nil, nil, nil, t.table),
		container.NewBorder(widget.NewLabel("Content Matches"), nil, nil, nil, container.NewVScroll(t.viewer)),
	)
	split.Offset = 0.4
	return split
}

func (t *detailsTab) updateCell(id widget.TableCellID, l *tappableLabel) {
	if id.Row >= len(t.order) {
		l.SetText("")
		return
	}
	res := t.app.results[t.order[id.Row]]
	switch id.Col {
	case 0:
		l.SetText(res.Name)
	case 1:
		l.SetText(formatModTime(res.ModTime))
	case 2:
		l.SetText(formatSize(res.Size))
	}
	l.OnTapped = func() { t.selectRow(id.Row) }
	l.OnDoubleTapped = func() { t.openRow(id.Row) }
	l.OnSecondaryTapped = func(e *fyne.PointEvent) {
		t.selectRow(id.Row)
		rec := t.recordFor(id.Row)
		menu := t.app.fileContextMenu(rec, func() { t.table.Refresh() })
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

func (t *detailsTab) recordFor(row int) fileRecord {
	res := t.app.results[t.order[row]]
	return fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
}

func (t *detailsTab) selectRow(row int) {
	t.selRow = row
	t.showMatches(t.app.results[t.order[row]])
}

func (t *detailsTab) openRow(row int) {
	res := t.app.results[t.order[row]]
	if err := t.app.openResult(res.Path); err != nil {
		t.app.setStatus("Failed to open: " + err.Error())
	}
}

var (
	matchStyle  = &widget.CustomTextGridStyle{FGColor: whiteColor, BGColor: brandBlue, TextStyle: fyne.TextStyle{Bold: true}}
	lineNoStyle = &widget.CustomTextGridStyle{FGColor: brandBlue, TextStyle: fyne.TextStyle{Bold: true}}
)

var whiteColor = &fyneColor{r: 0xFF, g: 0xFF, b: 0xFF, a: 0xFF}

// showMatches renders each ContentMatch block separated by a dashed line,
// each context line prefixed with a right-aligned line number, and
// highlights every content-regex occurrence on the actual match line --
// mirroring the original app's Result Details match viewer.
func (t *detailsTab) showMatches(res model.FileResult) {
	var b []byte
	type styledRange struct {
		row, start, end int
		style           widget.TextGridStyle
	}
	var ranges []styledRange

	row := 0
	for i, m := range res.Matches {
		if i > 0 {
			b = append(b, []byte("------------------------------------------------------------\n")...)
			row++
		}
		for li, line := range m.ContextLines {
			lineNum := m.ContextStartLine + li
			prefix := fmt.Sprintf("%4d: ", lineNum)
			b = append(b, prefix...)
			ranges = append(ranges, styledRange{row: row, start: 0, end: len(prefix), style: lineNoStyle})
			if lineNum == m.LineNum {
				if re := t.app.currentContentRegex(); re != nil {
					for _, loc := range re.FindAllStringIndex(line, -1) {
						ranges = append(ranges, styledRange{row: row, start: len(prefix) + loc[0], end: len(prefix) + loc[1], style: matchStyle})
					}
				}
			}
			b = append(b, line...)
			b = append(b, '\n')
			row++
		}
	}

	t.viewer.SetText(string(b))
	for _, r := range ranges {
		t.viewer.SetStyleRange(r.row, r.start, r.row, r.end, r.style)
	}
}

func (t *detailsTab) sortBy(col int) {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
		t.sortAsc = true
	}
	t.resort()
}

// resort rebuilds the display order from t.app.results, called both when
// the sort column/direction changes and whenever new results arrive.
func (t *detailsTab) resort() {
	order := make([]int, len(t.app.results))
	for i := range order {
		order[i] = i
	}
	results := t.app.results
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		var lt bool
		switch t.sortCol {
		case 1:
			lt = a.ModTime.Before(b.ModTime)
		case 2:
			lt = a.Size < b.Size
		default:
			lt = a.Name < b.Name
		}
		if !t.sortAsc {
			return !lt
		}
		return lt
	}
	sort.SliceStable(order, less)
	t.order = order
	t.table.Refresh()
}

func (t *detailsTab) clear() {
	t.order = nil
	t.selRow = -1
	t.viewer.SetText("")
	t.table.Refresh()
}
