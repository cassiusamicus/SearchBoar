package ui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// resultsView is the "list of filenames + full-detail cards with
// Prev/Next" component shared by the Detailed Results tab (which adds its
// own sort controls and Stop button around it) and the Start tab's
// compact results panel (fixed sort, no controls). Both are independent
// widget instances -- a Fyne widget can only live in one place in the
// tree at a time -- but both are views over the same app.searchResults,
// kept in sync because App fans addResult/resort/clear out to every
// active view (see searchcontrol.go).
type resultsView struct {
	app *App

	sortField string // "Name", "Location", "Modified", "Size", "Number of hits"
	sortAsc   bool
	order     []int // display index -> app.searchResults index
	selIdx    int   // "current" card (Prev/Next + highlight target), -1 if none

	countLabel *widget.Label
	prevBtn    *widget.Button
	nextBtn    *widget.Button

	list     *widget.List
	scroll   *container.Scroll
	cardsBox *fyne.Container
	cards    []*resultCard // parallel to order

	lastCardRefresh time.Time // throttles addResult's Refresh calls; see there
}

// resultCard is one result's card: a translucent highlight rectangle
// (shown only for the "current" card) stacked behind the actual content.
type resultCard struct {
	root      fyne.CanvasObject
	highlight *canvas.Rectangle
	resultIdx int // index into app.searchResults
}

func newResultsView(a *App, sortField string, sortAsc bool) *resultsView {
	return &resultsView{app: a, sortField: sortField, sortAsc: sortAsc, selIdx: -1}
}

// build constructs the list, cards, and Prev/Next/count widgets. Callers
// arrange these (v.list, v.scroll, v.prevBtn, v.nextBtn, v.countLabel)
// into their own layout -- a full tab with a header row of sort controls,
// or a compact panel with just Prev/Next above the cards.
func (v *resultsView) build() {
	v.countLabel = widget.NewLabel("")
	v.cardsBox = container.NewVBox()
	v.scroll = container.NewVScroll(v.cardsBox)

	// A plain widget.Label is used for list rows -- a custom Tappable
	// wrapper here previously intercepted every click before List's own
	// selection logic ran (Fyne dispatches taps to the deepest Tappable
	// under the pointer), leaving clicks not doing anything. Selection now
	// goes through List.OnSelected, the well-tested native path.
	v.list = widget.NewList(
		func() int { return len(v.order) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) { v.updateListRow(id, o.(*widget.Label)) },
	)
	v.list.OnSelected = func(id widget.ListItemID) {
		v.selIdx = id
		v.updateHighlight()
		v.scrollToCurrent()
		v.updateNavButtons()
	}

	v.prevBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() { v.step(-1) })
	v.nextBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() { v.step(1) })
	v.prevBtn.Importance = widget.LowImportance
	v.nextBtn.Importance = widget.LowImportance
	v.prevBtn.Disable()
	v.nextBtn.Disable()
}

func (v *resultsView) updateListRow(id widget.ListItemID, l *widget.Label) {
	if id >= len(v.order) {
		l.SetText("")
		return
	}
	l.SetText(v.app.searchResults[v.order[id]].Name)
}

// displayPath prefers a result's network-style DisplayPath (for files
// found under a mounted SMB/NFS share) over its raw local mount-point Path.
func displayPath(res model.FileResult) string {
	if res.DisplayPath != "" {
		return res.DisplayPath
	}
	return res.Path
}

// buildCard renders one result (identified by its index into
// app.searchResults, not its display position) as a self-contained card:
// filename, path/date/size, Open/Actions buttons, and every content match
// -- everything a click on a list row used to reveal, visible up front.
func (v *resultsView) buildCard(resultIdx int) *resultCard {
	res := v.app.searchResults[resultIdx]

	// Both wrapped: a filename or (especially) a directory path has no
	// guaranteed word-break points (paths use "/", which Fyne's word-wrap
	// doesn't treat as breakable), so without wrapping either one can
	// render as a single very long unbroken line -- and since Fyne sizes
	// a window to fit its content's minimum size, one long path anywhere
	// in the result list was enough to force the *whole window* wider
	// than the screen. TextWrapBreak (breaks at any character, not just
	// word boundaries) is what actually guarantees that regardless of
	// content; word-wrap alone doesn't help unbroken text like a path.
	title := widget.NewRichText(&widget.TextSegment{Text: res.Name, Style: widget.RichTextStyleSubHeading})
	title.Wrapping = fyne.TextWrapBreak
	meta := widget.NewLabel(fmt.Sprintf("%s   •   %s   •   %s", filepath.Dir(displayPath(res)), formatModTime(res.ModTime), formatSize(res.Size)))
	meta.Importance = widget.LowImportance
	meta.Wrapping = fyne.TextWrapBreak

	openBtn := widget.NewButtonWithIcon("Open", theme.MediaPlayIcon(), func() {
		if err := v.app.openResult(res.Path); err != nil {
			v.app.setStatus("Failed to open: " + err.Error())
		}
	})
	actionsBtn := widget.NewButtonWithIcon("Actions", theme.MoreVerticalIcon(), nil)
	openBtn.Importance = widget.LowImportance
	actionsBtn.Importance = widget.LowImportance
	actionsBtn.OnTapped = func() {
		rec := fileRecord{Path: res.Path, Name: res.Name, ModifiedStr: formatModTime(res.ModTime), Size: res.Size, SizeHuman: formatSize(res.Size)}
		menu := v.app.fileContextMenu(rec, nil)
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(actionsBtn)
		pos = pos.Add(fyne.NewPos(0, actionsBtn.Size().Height))
		widget.ShowPopUpMenuAtPosition(menu, v.app.win.Canvas(), pos)
	}

	titleRow := container.NewBorder(nil, nil, nil, container.NewHBox(openBtn, actionsBtn), title)

	re := v.app.currentContentRegex()
	blocks := []fyne.CanvasObject{titleRow, meta, widget.NewSeparator()}
	for i, m := range res.Matches {
		if i > 0 {
			blocks = append(blocks, widget.NewSeparator())
		}
		blocks = append(blocks, v.buildMatchBlock(m, re))
	}
	if len(res.Matches) == 0 {
		blocks = append(blocks, widget.NewLabel("(filename match only -- no content preview)"))
	}

	highlight := canvas.NewRectangle(color.Transparent)
	body := container.NewPadded(container.NewVBox(blocks...))

	return &resultCard{root: container.NewStack(highlight, body), highlight: highlight, resultIdx: resultIdx}
}

func (v *resultsView) buildMatchBlock(m model.ContentMatch, re *regexp.Regexp) fyne.CanvasObject {
	var rows []fyne.CanvasObject
	for li, line := range m.ContextLines {
		lineNum := m.ContextStartLine + li
		segs := []widget.RichTextSegment{
			&widget.TextSegment{Text: fmt.Sprintf("%4d:  ", lineNum), Style: widget.RichTextStyle{Inline: true, ColorName: theme.ColorNamePlaceHolder}},
		}
		// Highlighting used to be gated on lineNum == m.LineNum -- but a
		// ContentMatch's ContextLines can span several genuinely-matching
		// lines grouped into one block (consecutive matches within
		// gapThreshold of each other get merged, see runRipgrep/
		// matchLinesInSlice), with only the first one recorded as LineNum.
		// Any other real match line in the same block was silently never
		// highlighted, even though it does contain the term -- checking
		// every line against re instead (highlightSegments already no-ops
		// into plain text for a line with no match) finds all of them.
		if re != nil {
			segs = append(segs, highlightSegments(line, re)...)
		} else {
			segs = append(segs, &widget.TextSegment{Text: line, Style: widget.RichTextStyleInline})
		}
		rt := widget.NewRichText(segs...)
		rt.Wrapping = fyne.TextWrapWord
		rows = append(rows, rt)
	}
	return container.NewVBox(rows...)
}

// highlightSegments splits line into plain/bold-highlighted runs around
// every match of re.
func highlightSegments(line string, re *regexp.Regexp) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
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
	return segs
}

// step moves the "current" card by delta, scrolling it into view and
// highlighting it -- every card's content is already visible by scrolling
// manually, so this is a shortcut to jump straight to the next/previous
// one, not the only way to reach it.
func (v *resultsView) step(delta int) {
	next := v.selIdx + delta
	if next < 0 || next >= len(v.order) {
		return
	}
	v.selIdx = next
	v.updateHighlight()
	v.scrollToCurrent()
	v.updateNavButtons()
	v.list.Select(next)
}

// selectionColor is the theme's selection color, used to highlight the
// "current" card the same way a selected list row is highlighted elsewhere
// in the app.
func selectionColor() color.Color {
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	return fyne.CurrentApp().Settings().Theme().Color(theme.ColorNameSelection, variant)
}

func (v *resultsView) updateHighlight() {
	sel := selectionColor()
	for i, card := range v.cards {
		if i == v.selIdx {
			card.highlight.FillColor = sel
		} else {
			card.highlight.FillColor = color.Transparent
		}
		card.highlight.Refresh()
	}
}

func (v *resultsView) scrollToCurrent() {
	if v.selIdx < 0 || v.selIdx >= len(v.cards) {
		return
	}
	pos := v.cards[v.selIdx].root.Position()
	v.scroll.ScrollToOffset(fyne.NewPos(0, pos.Y))
}

func (v *resultsView) updateNavButtons() {
	if len(v.order) == 0 {
		v.prevBtn.Disable()
		v.nextBtn.Disable()
		return
	}
	if v.selIdx <= 0 {
		v.prevBtn.Disable()
	} else {
		v.prevBtn.Enable()
	}
	if v.selIdx >= len(v.order)-1 {
		v.nextBtn.Disable()
	} else {
		v.nextBtn.Enable()
	}
}

// resort rebuilds every card from app.searchResults in the current sort
// order -- called when the sort field/direction changes (Detailed Results
// only -- the Start tab's view has a fixed sort) and once when a search
// finishes (see finishSearch in searchcontrol.go). Rebuilding every card
// on every single incoming result (there can be many during a big search)
// would get slower as the result set grows, so addResult appends instead;
// the final sort is applied once here.
func (v *resultsView) resort() {
	selectedResult := -1
	if v.selIdx >= 0 && v.selIdx < len(v.order) {
		selectedResult = v.order[v.selIdx]
	}

	order := make([]int, len(v.app.searchResults))
	for i := range order {
		order[i] = i
	}
	results := v.app.searchResults
	// Each case returns the correct comparator for the current direction
	// directly, rather than computing the ascending result and negating it
	// for descending -- negation is wrong for tied elements (e.g. two
	// files with the same hit count): !(a < b) is true whenever a >= b,
	// so both less(i,j) and less(j,i) come back true for a tie, which
	// isn't a valid strict-weak-ordering and produced visibly wrong
	// positions (a 3-hit file sorting after two 1-hit files under
	// "Number of Hits, Descending").
	less := func(i, j int) bool {
		a, b := results[order[i]], results[order[j]]
		switch v.sortField {
		case "Location":
			da, db := filepath.Dir(displayPath(a)), filepath.Dir(displayPath(b))
			if v.sortAsc {
				return da < db
			}
			return da > db
		case "Modified":
			if v.sortAsc {
				return a.ModTime.Before(b.ModTime)
			}
			return a.ModTime.After(b.ModTime)
		case "Size":
			if v.sortAsc {
				return a.Size < b.Size
			}
			return a.Size > b.Size
		case "Number of hits":
			if v.sortAsc {
				return len(a.Matches) < len(b.Matches)
			}
			return len(a.Matches) > len(b.Matches)
		default:
			if v.sortAsc {
				return a.Name < b.Name
			}
			return a.Name > b.Name
		}
	}
	sort.SliceStable(order, less)
	v.order = order
	v.rebuildCards()
	v.list.Refresh()
	v.countLabel.SetText(fmt.Sprintf("%d result(s)", len(v.order)))

	if len(v.order) == 0 {
		v.selIdx = -1
		v.updateNavButtons()
		return
	}

	newIdx := 0
	if selectedResult >= 0 {
		for i, resultIdx := range v.order {
			if resultIdx == selectedResult {
				newIdx = i
				break
			}
		}
	}
	v.selIdx = newIdx
	v.updateHighlight()
	v.scrollToCurrent()
	v.updateNavButtons()
	v.list.Select(newIdx)
}

func (v *resultsView) rebuildCards() {
	v.cards = make([]*resultCard, 0, len(v.order))
	objs := make([]fyne.CanvasObject, 0, len(v.order)*2)
	for i, idx := range v.order {
		card := v.buildCard(idx)
		v.cards = append(v.cards, card)
		if i > 0 {
			objs = append(objs, widget.NewSeparator())
		}
		objs = append(objs, card.root)
	}
	v.cardsBox.Objects = objs
	v.cardsBox.Refresh()
}

// addResult appends one card in arrival order without re-sorting or
// rebuilding the rest -- see resort's doc comment for why.
func (v *resultsView) addResult(model.FileResult) {
	idx := len(v.app.searchResults) - 1
	v.order = append(v.order, idx)

	card := v.buildCard(idx)
	v.cards = append(v.cards, card)
	if len(v.cardsBox.Objects) > 0 {
		v.cardsBox.Objects = append(v.cardsBox.Objects, widget.NewSeparator())
	}
	v.cardsBox.Objects = append(v.cardsBox.Objects, card.root)

	// Refresh re-lays-out the whole (ever-growing) card VBox, which gets
	// more expensive the more results have already arrived -- calling it
	// on every single result makes total the cost of a large result set
	// scale roughly with the square of its size, and a fast search (many
	// small local files, say) can then queue results faster than the UI
	// thread can grind through that redraw cost. The visible symptom was
	// the app appearing to keep "still searching" -- new cards kept
	// appearing -- for a long time after the search itself (or a Stop
	// click) had already finished, since Fyne's UI work queue had a large
	// already-committed backlog of these redraws left to get through.
	// Throttled the same way engine.go throttles progress updates; safe to
	// skip here because resort() (always called when a search finishes,
	// see finishSearch) unconditionally rebuilds and refreshes every card
	// regardless, so nothing added during a throttled window is ever
	// permanently left unrendered.
	if time.Since(v.lastCardRefresh) >= 100*time.Millisecond {
		v.cardsBox.Refresh()
		v.list.Refresh()
		v.lastCardRefresh = time.Now()
	}

	v.countLabel.SetText(fmt.Sprintf("%d result(s)", len(v.order)))

	if v.selIdx < 0 {
		v.selIdx = 0
		v.updateHighlight()
	}
	v.updateNavButtons()
}

func (v *resultsView) clear() {
	v.order = nil
	v.cards = nil
	v.selIdx = -1
	if v.cardsBox != nil {
		v.cardsBox.Objects = nil
		v.cardsBox.Refresh()
	}
	if v.list != nil {
		v.list.UnselectAll()
		v.list.Refresh()
	}
	if v.countLabel != nil {
		v.countLabel.SetText("")
	}
	if v.prevBtn != nil {
		v.prevBtn.Disable()
	}
	if v.nextBtn != nil {
		v.nextBtn.Disable()
	}
}
