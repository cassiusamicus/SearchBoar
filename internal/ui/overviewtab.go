package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

const overviewPreviewLines = 3

type overviewTab struct {
	app   *App
	order []int // display index -> app.results index, in arrival order
	list  *widget.List
}

func newOverviewTab(a *App) *overviewTab {
	return &overviewTab{app: a}
}

func (t *overviewTab) build() fyne.CanvasObject {
	t.list = widget.NewList(
		func() int { return len(t.order) },
		func() fyne.CanvasObject {
			return newTappableBox(
				widget.NewRichTextWithText(""),
				widget.NewLabel(""),
				widget.NewRichText(),
				widget.NewSeparator(),
			)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) { t.updateCard(id, o.(*tappableBox)) },
	)
	return t.list
}

func (t *overviewTab) updateCard(id widget.ListItemID, box *tappableBox) {
	if id >= len(t.order) {
		return
	}
	res := t.app.results[t.order[id]]

	title := widget.NewRichTextWithText(res.Name)
	title.Segments[0].(*widget.TextSegment).Style = widget.RichTextStyleStrong

	subtitle := widget.NewLabel("Modified: " + formatModTime(res.ModTime))

	preview := widget.NewRichText(t.previewSegments(res)...)

	box.SetObjects([]fyne.CanvasObject{title, subtitle, preview, widget.NewSeparator()})
	box.OnTapped = func() { t.list.Select(id) }
	box.OnDoubleTapped = func() {
		if err := t.app.openResult(res.Path); err != nil {
			t.app.setStatus("Failed to open: " + err.Error())
		}
	}
	box.OnSecondaryTapped = func(e *fyne.PointEvent) {
		t.list.Select(id)
		rec := fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
		menu := t.app.fileContextMenu(rec, func() { t.list.Refresh() })
		widget.ShowPopUpMenuAtPosition(menu, t.app.win.Canvas(), e.AbsolutePosition)
	}
}

// previewSegments renders up to the first overviewPreviewLines lines of
// context around the first match, highlighting each content-regex
// occurrence, matching the original app's Overview card preview.
func (t *overviewTab) previewSegments(res model.FileResult) []widget.RichTextSegment {
	if len(res.Matches) == 0 {
		return nil
	}
	m := res.Matches[0]
	re := t.app.currentContentRegex()

	var segs []widget.RichTextSegment
	for i, line := range m.ContextLines {
		if i >= overviewPreviewLines {
			break
		}
		lineNum := m.ContextStartLine + i
		segs = append(segs, &widget.TextSegment{Text: fmt.Sprintf("Line %d: ", lineNum), Style: widget.RichTextStyleInline})

		if re == nil {
			segs = append(segs, &widget.TextSegment{Text: line, Style: widget.RichTextStyleInline})
			segs = append(segs, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyleInline})
			continue
		}
		last := 0
		for _, loc := range re.FindAllStringIndex(line, -1) {
			if loc[0] > last {
				segs = append(segs, &widget.TextSegment{Text: line[last:loc[0]], Style: widget.RichTextStyleInline})
			}
			segs = append(segs, &widget.TextSegment{Text: line[loc[0]:loc[1]], Style: widget.RichTextStyle{
				Inline:    true,
				TextStyle: fyne.TextStyle{Bold: true},
			}})
			last = loc[1]
		}
		if last < len(line) {
			segs = append(segs, &widget.TextSegment{Text: line[last:], Style: widget.RichTextStyleInline})
		}
		segs = append(segs, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyleInline})
	}
	return segs
}

func (t *overviewTab) addResult(res model.FileResult) {
	t.order = append(t.order, len(t.app.results)-1)
	t.list.Refresh()
}

func (t *overviewTab) clear() {
	t.order = nil
	t.list.Refresh()
}
