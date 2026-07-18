package ui

import (
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsTab is the unified "Results" tab: every match (local and
// network) in one sortable, icon-led list -- Name / Location / Modified /
// Size, styled after epicorg's FilePicker -- plus a content-match preview
// pane and the usual open/context-menu actions.
type resultsTab struct {
	app *App

	sortCol int // 0=name, 1=location, 2=modified, 3=size
	sortAsc bool
	order   []int
	selRow  int

	table  *widget.Table
	viewer *widget.TextGrid
}

func newResultsTab(a *App) *resultsTab {
	return &resultsTab{app: a, sortAsc: true, selRow: -1}
}

func (t *resultsTab) build() fyne.CanvasObject {
	t.table = widget.NewTable(
		func() (int, int) { return len(t.order), 4 },
		func() fyne.CanvasObject { return newTappableBox() },
		func(id widget.TableCellID, o fyne.CanvasObject) { t.updateCell(id, o.(*tappableBox)) },
	)
	t.table.ShowHeaderRow = true
	t.table.CreateHeader = func() fyne.CanvasObject { return widget.NewButton("", nil) }
	t.table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		b := o.(*widget.Button)
		headers := []string{"Name", "Location", "Modified", "Size"}
		b.SetText(headers[id.Col])
		col := id.Col
		b.OnTapped = func() { t.sortBy(col) }
	}
	t.table.SetColumnWidth(0, 220)
	t.table.SetColumnWidth(1, 260)
	t.table.SetColumnWidth(2, 150)
	t.table.SetColumnWidth(3, 90)

	t.viewer = widget.NewTextGrid()

	split := container.NewHSplit(
		container.NewBorder(widget.NewLabel("Files Found"), nil, nil, nil, t.table),
		container.NewBorder(widget.NewLabel("Content Matches"), nil, nil, nil, container.NewVScroll(t.viewer)),
	)
	split.Offset = 0.45
	return split
}

func (t *resultsTab) updateCell(id widget.TableCellID, box *tappableBox) {
	if id.Row >= len(t.order) {
		box.SetObjects(nil)
		return
	}
	res := t.app.searchResults[t.order[id.Row]]

	switch id.Col {
	case 0:
		icon := widget.NewIcon(theme.FileIcon())
		box.SetObjects([]fyne.CanvasObject{icon, widget.NewLabel(res.Name)})
	case 1:
		box.SetObjects([]fyne.CanvasObject{widget.NewLabel(filepath.Dir(displayPath(res)))})
	case 2:
		box.SetObjects([]fyne.CanvasObject{widget.NewLabel(formatModTime(res.ModTime))})
	case 3:
		box.SetObjects([]fyne.CanvasObject{widget.NewLabel(formatSize(res.Size))})
	}

	row := id.Row
	box.OnTapped = func() { t.selectRow(row) }
	box.OnDoubleTapped = func() { t.openRow(row) }
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		t.selectRow(row)
		rec := t.recordFor(row)
		menu := t.app.fileContextMenu(rec, func() { t.table.Refresh() })
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

// displayPath prefers a result's network-style DisplayPath (for files
// found under a mounted SMB/NFS share) over its raw local mount-point Path.
func displayPath(res model.FileResult) string {
	if res.DisplayPath != "" {
		return res.DisplayPath
	}
	return res.Path
}

func (t *resultsTab) recordFor(row int) fileRecord {
	res := t.app.searchResults[t.order[row]]
	return fileRecord{
		Path:        res.Path,
		Name:        res.Name,
		ModifiedStr: formatModTime(res.ModTime),
		Size:        res.Size,
		SizeHuman:   formatSize(res.Size),
	}
}

func (t *resultsTab) selectRow(row int) {
	t.selRow = row
	t.showMatches(t.app.searchResults[t.order[row]])
}

func (t *resultsTab) openRow(row int) {
	res := t.app.searchResults[t.order[row]]
	if err := t.app.openResult(res.Path); err != nil {
		t.app.setStatus("Failed to open: " + err.Error())
	}
}

var (
	matchStyle  = &widget.CustomTextGridStyle{FGColor: whiteColor, BGColor: brandBlue, TextStyle: fyne.TextStyle{Bold: true}}
	lineNoStyle = &widget.CustomTextGridStyle{FGColor: brandBlue, TextStyle: fyne.TextStyle{Bold: true}}
	whiteColor  = &fyneColor{r: 0xFF, g: 0xFF, b: 0xFF, a: 0xFF}
)

// showMatches renders each ContentMatch block separated by a dashed line,
// each context line prefixed with a right-aligned line number, and
// highlights every content-regex occurrence on the actual match line.
func (t *resultsTab) showMatches(res model.FileResult) {
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

func (t *resultsTab) sortBy(col int) {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
		t.sortAsc = true
	}
	t.resort()
}

// resort rebuilds the display order from t.app.searchResults, called both
// when the sort column/direction changes and whenever new results arrive.
func (t *resultsTab) resort() {
	order := make([]int, len(t.app.searchResults))
	for i := range order {
		order[i] = i
	}
	results := t.app.searchResults
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		var lt bool
		switch t.sortCol {
		case 1:
			lt = filepath.Dir(displayPath(a)) < filepath.Dir(displayPath(b))
		case 2:
			lt = a.ModTime.Before(b.ModTime)
		case 3:
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

func (t *resultsTab) addResult(model.FileResult) {
	t.resort()
}

func (t *resultsTab) clear() {
	t.order = nil
	t.selRow = -1
	t.viewer.SetText("")
	t.table.Refresh()
}
