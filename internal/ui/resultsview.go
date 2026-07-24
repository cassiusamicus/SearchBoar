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
	selIdx    int   // "current" card/file (outer Rewind/Fast-Forward + card border), -1 if none
	curMatch  int   // "current" term instance within v.cards[selIdx] (inner Back/Forward), -1 if none

	countLabel *widget.Label
	// prevBtn/nextBtn (outer, Rewind/Fast-Forward icons) step whole files;
	// matchPrevBtn/matchNextBtn (inner, plain Back/Forward icons) step one
	// term instance at a time within/across files -- a tape recorder's
	// fast-forward-vs-single-step distinction, matching how differently
	// large a jump each pair makes. jumpTopBtn/jumpBottomBtn skip straight
	// to the very first/last result, for a long result list where even
	// fast-forwarding file-by-file is too slow to reach either end.
	prevBtn, nextBtn           *iconTipButton
	matchPrevBtn, matchNextBtn *iconTipButton
	jumpTopBtn, jumpBottomBtn  *iconTipButton

	list     *widget.List
	scroll   *container.Scroll
	cardsBox *fyne.Container
	cards    []*resultCard // parallel to order

	lastCardRefresh time.Time // throttles addResult's Refresh calls; see there
}

// resultCard is one result's card: a bordered highlight rectangle (shown
// only for the "current" card) stacked behind the actual content, plus one
// bordered frame per content match (matchFrames, parallel to the result's
// Matches) marking whichever one is the "current" term instance.
type resultCard struct {
	root        fyne.CanvasObject
	highlight   *canvas.Rectangle
	matchFrames []*canvas.Rectangle // parallel to app.searchResults[resultIdx].Matches
	resultIdx   int                 // index into app.searchResults
}

func newResultsView(a *App, sortField string, sortAsc bool) *resultsView {
	return &resultsView{app: a, sortField: sortField, sortAsc: sortAsc, selIdx: -1, curMatch: -1}
}

// build constructs the list, cards, and nav/count widgets. Callers arrange
// these (v.list, v.scroll, v.prevBtn/nextBtn, v.matchPrevBtn/matchNextBtn,
// v.countLabel) into their own layout -- a full tab with a header row of
// sort controls, or a compact panel with just the nav buttons above the
// cards.
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
		v.resetCurMatch()
		v.updateHighlight()
		v.scrollToCurrent()
		v.updateNavButtons()
	}

	v.prevBtn = newIconTipButton(theme.MediaFastRewindIcon(), "Previous file", v.app.win, func() { v.step(-1) })
	v.nextBtn = newIconTipButton(theme.MediaFastForwardIcon(), "Next file", v.app.win, func() { v.step(1) })
	v.matchPrevBtn = newIconTipButton(theme.NavigateBackIcon(), "Previous match", v.app.win, func() { v.stepMatch(-1) })
	v.matchNextBtn = newIconTipButton(theme.NavigateNextIcon(), "Next match", v.app.win, func() { v.stepMatch(1) })
	v.jumpTopBtn = newIconTipButton(theme.MoveUpIcon(), "Jump to first result", v.app.win, func() { v.jumpToFirst() })
	v.jumpBottomBtn = newIconTipButton(theme.MoveDownIcon(), "Jump to last result", v.app.win, func() { v.jumpToLast() })
	v.prevBtn.Disable()
	v.nextBtn.Disable()
	v.matchPrevBtn.Disable()
	v.matchNextBtn.Disable()
	v.jumpTopBtn.Disable()
	v.jumpBottomBtn.Disable()
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
	// Marked as secondary by size and slant, not by color: LowImportance
	// (the default "de-emphasized" choice) renders in ColorNameDisabled,
	// which in this theme is a mid-gray tuned to read as merely subtle
	// against a *light* background, but against this card's dark
	// background it's low-contrast enough to be hard to read outright, not
	// just subtle. Regular foreground color stays legible in both modes;
	// smaller + italic still reads clearly as "secondary, not the title or
	// match text" without sacrificing that legibility.
	meta := widget.NewLabel(fmt.Sprintf("%s   •   %s   •   %s", filepath.Dir(displayPath(res)), formatModTime(res.ModTime), formatSize(res.Size)))
	meta.Importance = widget.MediumImportance
	meta.SizeName = theme.SizeNameCaptionText
	meta.TextStyle = fyne.TextStyle{Italic: true}
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
	matchFrames := make([]*canvas.Rectangle, len(res.Matches))
	for i, m := range res.Matches {
		if i > 0 {
			blocks = append(blocks, widget.NewSeparator())
		}
		block, frame := v.buildMatchBlock(m, re)
		matchFrames[i] = frame
		blocks = append(blocks, block)
	}
	if len(res.Matches) == 0 {
		blocks = append(blocks, widget.NewLabel("(filename match only -- no content preview)"))
	}

	highlight := canvas.NewRectangle(color.Transparent)
	body := container.NewPadded(container.NewVBox(blocks...))

	return &resultCard{
		root: container.NewStack(highlight, body), highlight: highlight,
		matchFrames: matchFrames, resultIdx: resultIdx,
	}
}

// buildMatchBlock renders one content match's context lines, and returns a
// frame rectangle stacked behind them -- transparent until updateHighlight
// marks this specific match as the "current" one (the inner Back/Forward
// buttons' target), giving each term instance its own "you are here"
// indicator distinct from every other instance on screen, in the same
// card or elsewhere -- separate from, and complementing, the per-word
// highlight marks every match already carries (see highlightMarkSegment).
func (v *resultsView) buildMatchBlock(m model.ContentMatch, re *regexp.Regexp) (fyne.CanvasObject, *canvas.Rectangle) {
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
	frame := canvas.NewRectangle(color.Transparent)
	content := container.NewPadded(container.NewVBox(rows...))
	return container.NewStack(frame, content), frame
}

// highlightSegments splits line into plain/highlighted runs around every
// match of re, using highlightMarkSegment (a background-marker, not just
// colored/bold text -- see its own doc comment for why) for the matches.
func highlightSegments(line string, re *regexp.Regexp) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
	last := 0
	for _, loc := range re.FindAllStringIndex(line, -1) {
		if loc[0] > last {
			segs = append(segs, &widget.TextSegment{Text: line[last:loc[0]], Style: widget.RichTextStyleInline})
		}
		segs = append(segs, &highlightMarkSegment{text: line[loc[0]:loc[1]]})
		last = loc[1]
	}
	if last < len(line) {
		segs = append(segs, &widget.TextSegment{Text: line[last:], Style: widget.RichTextStyleInline})
	}
	return segs
}

// highlightMarkSegment renders a run of matched text with an opaque
// background behind it, like a highlighter pen, instead of just colored
// bold text. Colored text alone wasn't enough: the accent (even
// contrast-adjusted via effectivePrimary for legibility against dark
// panels) and the card body's own foreground text color can end up close
// enough in brightness that bold+color barely reads as different from the
// surrounding text. widget.RichTextStyle has no public background field --
// the one Fyne uses internally for inline code is hardcoded to
// ColorNameInputBackground, a deliberately neutral color not meant to
// stand out -- so this implements a background+tight-fit layout directly,
// the same technique Fyne's own inline-code segment uses internally.
// Background/text are effectivePrimary/ForegroundOnPrimary, the same
// guaranteed-readable pairing HighImportance buttons use, so the mark
// stays legible no matter what accent color is chosen.
type highlightMarkSegment struct {
	text string
}

func (h *highlightMarkSegment) Inline() bool                        { return true }
func (h *highlightMarkSegment) Textual() string                     { return h.text }
func (h *highlightMarkSegment) Select(fyne.Position, fyne.Position) {}
func (h *highlightMarkSegment) SelectedText() string                { return "" }
func (h *highlightMarkSegment) Unselect()                           {}

func (h *highlightMarkSegment) Visual() fyne.CanvasObject {
	bg := canvas.NewRectangle(color.Transparent)
	txt := canvas.NewText(h.text, color.Transparent)
	txt.TextStyle = fyne.TextStyle{Bold: true}
	c := &fyne.Container{Layout: highlightMarkLayout{}, Objects: []fyne.CanvasObject{bg, txt}}
	h.Update(c)
	return c
}

func (h *highlightMarkSegment) Update(o fyne.CanvasObject) {
	c := o.(*fyne.Container)
	bg := c.Objects[0].(*canvas.Rectangle)
	txt := c.Objects[1].(*canvas.Text)

	th := fyne.CurrentApp().Settings().Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()
	bg.FillColor = th.Color(theme.ColorNamePrimary, v)
	bg.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	bg.Refresh()
	txt.Text = h.text
	txt.Color = th.Color(theme.ColorNameForegroundOnPrimary, v)
	txt.TextStyle = fyne.TextStyle{Bold: true}
	txt.Refresh()
}

// highlightMarkLayout keeps the background tight to the text, mirroring
// Fyne's own unexported codeInlineLayout (used for the same purpose by
// inline code segments): without it, a row layout stretching the segment
// to fill trailing space would stretch the highlight fill along with it.
type highlightMarkLayout struct{}

func (highlightMarkLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	return o[1].MinSize()
}

func (highlightMarkLayout) Layout(o []fyne.CanvasObject, _ fyne.Size) {
	size := o[1].MinSize()
	for _, obj := range o {
		obj.Resize(size)
		obj.Move(fyne.NewPos(0, 0))
	}
}

// step moves the "current" card (file) by delta -- the outer Rewind/
// Fast-Forward buttons. Every card's content is already visible by
// scrolling manually, so this is a shortcut to jump straight to the next/
// previous file, not the only way to reach it.
func (v *resultsView) step(delta int) {
	v.jumpTo(v.selIdx + delta)
}

// jumpToFirst/jumpToLast skip straight to the very first/last result --
// for a long result list, faster than fast-forwarding through every file
// to reach either end.
func (v *resultsView) jumpToFirst() { v.jumpTo(0) }
func (v *resultsView) jumpToLast()  { v.jumpTo(len(v.order) - 1) }

// jumpTo moves the "current" card straight to idx (a no-op if out of
// range -- covers both step's relative moves and jumpToFirst/jumpToLast's
// absolute ones, including on an empty result set, where idx is never in
// [0, len(v.order)) regardless of direction), scrolling it into view and
// bordering it, and resets the current term instance to the new card's
// first match (see stepMatch for stepping through instances one at a
// time, without changing files).
func (v *resultsView) jumpTo(idx int) {
	if idx < 0 || idx >= len(v.order) {
		return
	}
	v.selIdx = idx
	v.resetCurMatch()
	v.updateHighlight()
	v.scrollToCurrent()
	v.updateNavButtons()
	v.list.Select(idx)
}

// resetCurMatch points curMatch at the current card's first match, or -1
// if it has none (a filename-only hit) -- called whenever the current
// card changes by any means other than stepMatch itself, so a fresh card
// always starts at its first instance rather than carrying over whatever
// match index happened to be current on the previous card.
func (v *resultsView) resetCurMatch() {
	if len(v.matchesOf(v.selIdx)) == 0 {
		v.curMatch = -1
		return
	}
	v.curMatch = 0
}

// matchesOf returns cardIdx's content matches, or nil if cardIdx is out of
// range -- out-of-range is treated as "no matches" rather than a caller
// error throughout this file specifically so findMatch's boundary-probing
// (checking one card past either end) doesn't need its own special case.
func (v *resultsView) matchesOf(cardIdx int) []model.ContentMatch {
	if cardIdx < 0 || cardIdx >= len(v.order) {
		return nil
	}
	return v.app.searchResults[v.order[cardIdx]].Matches
}

// findMatch searches from the current position in the given direction
// (+1 or -1) for the next reachable term instance, moving into the next/
// previous file once the current file's matches are exhausted and
// skipping any file with no content matches at all (a filename-only hit
// has nothing to step to). ok is false once there's nothing further in
// that direction.
func (v *resultsView) findMatch(delta int) (cardIdx, matchIdx int, ok bool) {
	cardIdx = v.selIdx
	matchIdx = v.curMatch + delta
	for {
		if cardIdx < 0 || cardIdx >= len(v.order) {
			return 0, 0, false
		}
		if matches := v.matchesOf(cardIdx); matchIdx >= 0 && matchIdx < len(matches) {
			return cardIdx, matchIdx, true
		}
		if delta < 0 {
			cardIdx--
			matchIdx = len(v.matchesOf(cardIdx)) - 1
		} else {
			cardIdx++
			matchIdx = 0
		}
	}
}

// stepMatch moves the "current" term instance by delta -- the inner Back/
// Forward buttons -- scrolling straight to that specific match block and
// framing it (see updateHighlight), crossing into the next/previous file
// once the current file's matches run out (see findMatch).
func (v *resultsView) stepMatch(delta int) {
	cardIdx, matchIdx, ok := v.findMatch(delta)
	if !ok {
		return
	}
	fileChanged := cardIdx != v.selIdx
	v.selIdx = cardIdx
	v.curMatch = matchIdx
	v.updateHighlight()
	v.scrollToCurrentMatch()
	v.updateNavButtons()
	if fileChanged {
		v.list.Select(cardIdx)
	}
}

func (v *resultsView) updateHighlight() {
	th := fyne.CurrentApp().Settings().Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	// A solid accent-colored frame, not a translucent fill: with several
	// cards visible in the scroll area at once (a tall window, or several
	// short results), a subtle background tint on the current one was easy
	// to miss next to the others -- especially now that individual matched
	// terms inside every card also carry a background mark (see
	// highlightMarkSegment), which a whole-card tint in the same color
	// family would just blend into. A clearly bordered frame reads as "the
	// current card" unambiguously regardless of how many others are
	// visible, and doesn't compete visually with the in-text highlights.
	borderColor := th.Color(theme.ColorNamePrimary, variant)
	matchFillColor := translucent(borderColor, 0x30)
	for i, card := range v.cards {
		if i == v.selIdx {
			card.highlight.StrokeColor = borderColor
			card.highlight.StrokeWidth = 3
		} else {
			card.highlight.StrokeColor = color.Transparent
			card.highlight.StrokeWidth = 0
		}
		card.highlight.Refresh()

		// Same idea, one level down: within the current card, exactly one
		// match block -- the one the inner buttons last landed on -- gets
		// its own frame, so which specific instance you've stepped to
		// stays obvious even when several are visible in the same card at
		// once (a tinted fill here, not just a border, since a match block
		// is small enough that a border alone is easy to lose against the
		// surrounding text; the card-level frame above stays border-only
		// since a fill across a whole card would fight with the in-text
		// highlight marks it contains).
		for mi, frame := range card.matchFrames {
			if i == v.selIdx && mi == v.curMatch {
				frame.StrokeColor = borderColor
				frame.StrokeWidth = 2
				frame.FillColor = matchFillColor
			} else {
				frame.StrokeColor = color.Transparent
				frame.StrokeWidth = 0
				frame.FillColor = color.Transparent
			}
			frame.Refresh()
		}
	}
}

// translucent returns c with its alpha replaced by alpha, preserving hue.
func translucent(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

func (v *resultsView) scrollToCurrent() {
	if v.selIdx < 0 || v.selIdx >= len(v.cards) {
		return
	}
	pos := v.cards[v.selIdx].root.Position()
	v.scroll.ScrollToOffset(fyne.NewPos(0, pos.Y))
}

// scrollToCurrentMatch scrolls to the specific match block curMatch points
// at, not just its card -- important for a card with many matches spread
// across a tall scroll area. card.matchFrames[i] is nested several
// containers deep inside card.root, so unlike scrollToCurrent (where
// card.root is cardsBox's direct child and its own Position() is already
// cardsBox-relative), the offset needs computing via absolute screen
// positions instead.
func (v *resultsView) scrollToCurrentMatch() {
	if v.selIdx < 0 || v.selIdx >= len(v.cards) {
		return
	}
	card := v.cards[v.selIdx]
	if v.curMatch < 0 || v.curMatch >= len(card.matchFrames) {
		v.scrollToCurrent()
		return
	}
	driver := fyne.CurrentApp().Driver()
	framePos := driver.AbsolutePositionForObject(card.matchFrames[v.curMatch])
	contentPos := driver.AbsolutePositionForObject(v.cardsBox)
	v.scroll.ScrollToOffset(fyne.NewPos(0, framePos.Y-contentPos.Y))
}

func (v *resultsView) updateNavButtons() {
	if len(v.order) == 0 {
		v.prevBtn.Disable()
		v.nextBtn.Disable()
		v.matchPrevBtn.Disable()
		v.matchNextBtn.Disable()
		v.jumpTopBtn.Disable()
		v.jumpBottomBtn.Disable()
		return
	}
	if v.selIdx <= 0 {
		v.prevBtn.Disable()
		v.jumpTopBtn.Disable()
	} else {
		v.prevBtn.Enable()
		v.jumpTopBtn.Enable()
	}
	if v.selIdx >= len(v.order)-1 {
		v.nextBtn.Disable()
		v.jumpBottomBtn.Disable()
	} else {
		v.nextBtn.Enable()
		v.jumpBottomBtn.Enable()
	}
	if _, _, ok := v.findMatch(-1); ok {
		v.matchPrevBtn.Enable()
	} else {
		v.matchPrevBtn.Disable()
	}
	if _, _, ok := v.findMatch(1); ok {
		v.matchNextBtn.Enable()
	} else {
		v.matchNextBtn.Disable()
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
		v.curMatch = -1
		v.updateNavButtons()
		return
	}

	// Re-sorting changes which position a file is at, not the file itself
	// or its matches -- so if the previously-selected file is still found,
	// curMatch (an index into that same file's Matches) is still valid and
	// left alone. Only reset it if there was no previous selection, or the
	// file it pointed to is gone (fell back to newIdx's zero value, a
	// different file than whatever was selected).
	newIdx := 0
	found := false
	if selectedResult >= 0 {
		for i, resultIdx := range v.order {
			if resultIdx == selectedResult {
				newIdx = i
				found = true
				break
			}
		}
	}
	v.selIdx = newIdx
	if !found {
		v.resetCurMatch()
	}
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
		v.resetCurMatch()
		v.updateHighlight()
	}
	v.updateNavButtons()
}

func (v *resultsView) clear() {
	v.order = nil
	v.cards = nil
	v.selIdx = -1
	v.curMatch = -1
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
	if v.matchPrevBtn != nil {
		v.matchPrevBtn.Disable()
	}
	if v.matchNextBtn != nil {
		v.matchNextBtn.Disable()
	}
	if v.jumpTopBtn != nil {
		v.jumpTopBtn.Disable()
	}
	if v.jumpBottomBtn != nil {
		v.jumpBottomBtn.Disable()
	}
}
