package ui

import (
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsTab is the unified "Results" tab: a single scrolling column of
// self-contained cards (name, location/date/size, and a wrapped preview of
// the first match with highlighting), one per matched file -- modeled
// after epicorg's search results list rather than a file-table-plus-
// separate-preview-pane split. A table+narrow-preview-column split was
// tried first and abandoned: the preview column was too narrow to be
// useful, and widget.TextGrid (used for the preview) cannot wrap text at
// all, only truncate.
type resultsTab struct {
	app *App

	sortField string // "Name", "Location", "Modified", "Size"
	sortAsc   bool
	order     []int // display index -> app.searchResults index

	cardsBox   *fyne.Container
	countLabel *widget.Label
}

func newResultsTab(a *App) *resultsTab {
	return &resultsTab{app: a, sortField: "Name", sortAsc: true}
}

func (t *resultsTab) build() fyne.CanvasObject {
	// t.cardsBox/t.countLabel must exist before sortSelect.SetSelected
	// below, since Select.SetSelected fires its OnChanged synchronously
	// when the value actually changes, which calls resort() ->
	// rebuildCards() -> both of these.
	t.countLabel = widget.NewLabel("")
	t.cardsBox = container.NewVBox()

	sortSelect := widget.NewSelect([]string{"Name", "Location", "Modified", "Size"}, func(v string) {
		t.sortField = v
		t.resort()
	})
	sortSelect.SetSelected(t.sortField)

	dirBtn := widget.NewButton("↑ Ascending", nil)
	dirBtn.OnTapped = func() {
		t.sortAsc = !t.sortAsc
		if t.sortAsc {
			dirBtn.SetText("↑ Ascending")
		} else {
			dirBtn.SetText("↓ Descending")
		}
		t.resort()
	}

	header := container.NewHBox(widget.NewLabel("Sort by:"), sortSelect, dirBtn, layout.NewSpacer(), t.countLabel)

	return container.NewBorder(header, nil, nil, nil, container.NewVScroll(t.cardsBox))
}

// displayPath prefers a result's network-style DisplayPath (for files
// found under a mounted SMB/NFS share) over its raw local mount-point Path.
func displayPath(res model.FileResult) string {
	if res.DisplayPath != "" {
		return res.DisplayPath
	}
	return res.Path
}

func (t *resultsTab) buildCard(idx int) fyne.CanvasObject {
	res := t.app.searchResults[idx]

	title := widget.NewRichTextWithText(res.Name)
	title.Segments[0].(*widget.TextSegment).Style = widget.RichTextStyleStrong

	meta := widget.NewLabel(fmt.Sprintf("%s   •   %s   •   %s", filepath.Dir(displayPath(res)), formatModTime(res.ModTime), formatSize(res.Size)))

	objects := []fyne.CanvasObject{title, meta}
	if len(res.Matches) > 0 {
		preview := widget.NewRichText(t.previewSegments(res)...)
		preview.Wrapping = fyne.TextWrapWord
		objects = append(objects, preview)
	}
	objects = append(objects, widget.NewSeparator())

	box := newTappableBox(container.NewVBox(objects...))
	box.OnDoubleTapped = func() {
		if err := t.app.openResult(res.Path); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	}
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		rec := fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
		menu := t.app.fileContextMenu(rec, func() { t.resort() })
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
	return box
}

// previewSegments renders the first match's context lines as one wrapped,
// highlighted snippet (consecutive lines joined with a space rather than
// kept as separate hard-wrapped rows, so the card reflows naturally at any
// window width), with a "+N more match(es)" note if there were others.
func (t *resultsTab) previewSegments(res model.FileResult) []widget.RichTextSegment {
	if len(res.Matches) == 0 {
		return nil
	}
	m := res.Matches[0]
	re := t.app.currentContentRegex()

	segs := []widget.RichTextSegment{
		&widget.TextSegment{Text: fmt.Sprintf("Line %d:  ", m.LineNum), Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder}},
	}

	for i, line := range m.ContextLines {
		if i > 0 {
			segs = append(segs, &widget.TextSegment{Text: "  ", Style: widget.RichTextStyleInline})
		}
		if re == nil {
			segs = append(segs, &widget.TextSegment{Text: line, Style: widget.RichTextStyleInline})
			continue
		}
		last := 0
		for _, loc := range re.FindAllStringIndex(line, -1) {
			if loc[0] > last {
				segs = append(segs, &widget.TextSegment{Text: line[last:loc[0]], Style: widget.RichTextStyleInline})
			}
			segs = append(segs, &widget.TextSegment{
				Text:  line[loc[0]:loc[1]],
				Style: widget.RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}, ColorName: theme.ColorNamePrimary},
			})
			last = loc[1]
		}
		if last < len(line) {
			segs = append(segs, &widget.TextSegment{Text: line[last:], Style: widget.RichTextStyleInline})
		}
	}

	if len(res.Matches) > 1 {
		segs = append(segs, &widget.TextSegment{
			Text:  fmt.Sprintf("   (+%d more match%s)", len(res.Matches)-1, pluralS(len(res.Matches)-1)),
			Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder},
		})
	}
	return segs
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
}

// resort rebuilds the display order and every card from t.app.searchResults,
// called both when the sort field/direction changes and whenever a new
// result arrives.
func (t *resultsTab) resort() {
	order := make([]int, len(t.app.searchResults))
	for i := range order {
		order[i] = i
	}
	results := t.app.searchResults
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		var lt bool
		switch t.sortField {
		case "Location":
			lt = filepath.Dir(displayPath(a)) < filepath.Dir(displayPath(b))
		case "Modified":
			lt = a.ModTime.Before(b.ModTime)
		case "Size":
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
	t.rebuildCards()
}

func (t *resultsTab) rebuildCards() {
	objects := make([]fyne.CanvasObject, 0, len(t.order))
	for _, idx := range t.order {
		objects = append(objects, t.buildCard(idx))
	}
	t.cardsBox.Objects = objects
	t.cardsBox.Refresh()
	t.countLabel.SetText(fmt.Sprintf("%d result(s)", len(t.order)))
}

func (t *resultsTab) addResult(model.FileResult) {
	t.resort()
}

func (t *resultsTab) clear() {
	t.order = nil
	if t.cardsBox != nil {
		t.cardsBox.Objects = nil
		t.cardsBox.Refresh()
	}
	if t.countLabel != nil {
		t.countLabel.SetText("")
	}
}
