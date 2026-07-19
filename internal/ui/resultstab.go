package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsTab is the "Detailed Results" tab: a compact filename list on the
// left for quick navigation/overview, and every result rendered as a full
// card (filename, path/date/size, every content match highlighted and
// wrapped) stacked in one scrolling column on the right -- closest to
// epicorg's results view, but keeping the list as a second, quicker way to
// jump to a specific result instead of only scrolling. A Prev/Next pair in
// the header pages through every card -- scrolling the next/previous one
// into view and highlighting it (and syncing the list's own selection) --
// without requiring a click on anything; the buttons live in the header,
// above the list, since they page through every result found, not through
// matches inside one file.
//
// The list+cards+Prev/Next machinery itself lives in resultsview.go
// (resultsView), shared with the Start tab's compact results panel; this
// type just adds the sort controls and Stop button around it.
//
// Earlier versions used widget.TextGrid (can't wrap text at all), then a
// list-on-the-left/preview-on-the-right split where the right side only
// ever showed the one selected result, then briefly a cards-only layout
// with no list at all (dropping a feature that was still wanted).
type resultsTab struct {
	app  *App
	view *resultsView

	stopBtn *widget.Button
}

func newResultsTab(a *App) *resultsTab {
	// Descending by default: sorting by hit count is most useful with the
	// most-matched files first, unlike the other fields (name/location/
	// date), where ascending is the more natural default.
	return &resultsTab{app: a, view: newResultsView(a, "Number of hits", false)}
}

func (t *resultsTab) build() fyne.CanvasObject {
	// view.build() must run before sortSelect.SetSelected below, since
	// Select.SetSelected fires its OnChanged synchronously when the value
	// actually differs from "".
	t.view.build()

	sortSelect := widget.NewSelect([]string{"Number of hits", "Name", "Location", "Modified", "Size"}, func(v string) {
		t.view.sortField = v
		t.view.resort()
	})
	sortSelect.SetSelected(t.view.sortField)

	dirBtnText := func() string {
		if t.view.sortAsc {
			return "↑ Ascending"
		}
		return "↓ Descending"
	}
	dirBtn := widget.NewButton(dirBtnText(), nil)
	dirBtn.OnTapped = func() {
		t.view.sortAsc = !t.view.sortAsc
		if t.view.sortAsc {
			dirBtn.SetText("↑ Ascending")
		} else {
			dirBtn.SetText("↓ Descending")
		}
		t.view.resort()
	}

	t.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() { t.app.stopSearch() })
	t.stopBtn.Disable()

	header := container.NewHBox(
		widget.NewLabel("Sort by:"), sortSelect, dirBtn,
		widget.NewSeparator(),
		t.view.prevBtn, t.view.nextBtn,
		t.stopBtn,
		layout.NewSpacer(),
		t.view.countLabel,
	)

	split := container.NewHSplit(t.view.list, t.view.scroll)
	split.Offset = 0.22

	// HSplit's own MinSize is the *sum* of both children's widths; wrapping
	// in HScroll stops that from adding directly onto the window's minimum
	// width (see the longer explanation in starttab.go, which has the same
	// wrapper around its own split).
	return container.NewBorder(header, nil, nil, nil, container.NewHScroll(split))
}

func (t *resultsTab) addResult(r model.FileResult) { t.view.addResult(r) }
func (t *resultsTab) resort()                      { t.view.resort() }
func (t *resultsTab) clear()                       { t.view.clear() }
