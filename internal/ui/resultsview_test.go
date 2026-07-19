package ui

import (
	"regexp"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

func TestHighlightSegmentsMarksEveryOccurrence(t *testing.T) {
	re := regexp.MustCompile(`(?i)target`)
	segs := highlightSegments("target one, another target here", re)

	var marked []string
	for _, s := range segs {
		if hs, ok := s.(*highlightMarkSegment); ok {
			marked = append(marked, hs.text)
		}
	}
	if len(marked) != 2 || marked[0] != "target" || marked[1] != "target" {
		t.Errorf("marked segments = %v, want [\"target\" \"target\"]", marked)
	}
}

func TestHighlightSegmentsNoMatchStaysPlain(t *testing.T) {
	re := regexp.MustCompile(`target`)
	segs := highlightSegments("nothing to see here", re)
	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1 (no match -> whole line as one plain segment)", len(segs))
	}
	if _, ok := segs[0].(*highlightMarkSegment); ok {
		t.Errorf("segment = %+v, want a plain TextSegment, not a highlight mark", segs[0])
	}
}

// TestBuildMatchBlockHighlightsEveryMatchingLineInABlock guards against a
// real bug: ripgrep/the walk engine groups consecutive matching lines within
// gapThreshold of each other into one ContentMatch, recording only the
// first as LineNum -- highlighting used to be gated on lineNum == m.LineNum,
// so any other line in that same block that also genuinely contained the
// search term was silently never highlighted.
func TestBuildMatchBlockHighlightsEveryMatchingLineInABlock(t *testing.T) {
	v := &resultsView{}
	re := regexp.MustCompile(`target`)
	m := model.ContentMatch{
		LineNum:          2,
		ContextStartLine: 1,
		ContextLines: []string{
			"line one, no match here",
			"first target line",
			"second target line, also a real match",
		},
	}

	block, frame := v.buildMatchBlock(m, re)
	if frame == nil {
		t.Fatal("buildMatchBlock returned a nil frame rectangle")
	}
	// block is Stack(frame, Padded(VBox(rows...))) -- see buildMatchBlock.
	stack, ok := block.(*fyne.Container)
	if !ok || len(stack.Objects) != 2 {
		t.Fatalf("buildMatchBlock content = %T, want a 2-child *fyne.Container (frame, padded rows)", block)
	}
	padded, ok := stack.Objects[1].(*fyne.Container)
	if !ok || len(padded.Objects) != 1 {
		t.Fatalf("stack.Objects[1] = %T, want a 1-child *fyne.Container (Padded)", stack.Objects[1])
	}
	vbox, ok := padded.Objects[0].(*fyne.Container)
	if !ok {
		t.Fatalf("padded.Objects[0] = %T, want *fyne.Container (VBox of rows)", padded.Objects[0])
	}
	if len(vbox.Objects) != 3 {
		t.Fatalf("got %d rows, want 3", len(vbox.Objects))
	}
	rows := make([]*widget.RichText, len(vbox.Objects))
	for i, o := range vbox.Objects {
		rt, ok := o.(*widget.RichText)
		if !ok {
			t.Fatalf("row %d is %T, want *widget.RichText", i, o)
		}
		rows[i] = rt
	}

	markCountInRow := func(row *widget.RichText) int {
		n := 0
		for _, s := range row.Segments {
			if _, ok := s.(*highlightMarkSegment); ok {
				n++
			}
		}
		return n
	}

	if markCountInRow(rows[0]) != 0 {
		t.Errorf("row 0 (no match) has highlight marks, want none")
	}
	if markCountInRow(rows[1]) != 1 {
		t.Errorf("row 1 (LineNum, has a real match) mark count = %d, want 1", markCountInRow(rows[1]))
	}
	if markCountInRow(rows[2]) != 1 {
		t.Errorf("row 2 (not LineNum, but also a real match) mark count = %d, want 1 -- this is the bug this test guards against", markCountInRow(rows[2]))
	}
}

func TestFindMatchStepsWithinCard(t *testing.T) {
	app := &App{searchResults: []model.FileResult{
		{Matches: make([]model.ContentMatch, 3)},
	}}
	v := &resultsView{app: app, order: []int{0}, selIdx: 0, curMatch: 0}

	cardIdx, matchIdx, ok := v.findMatch(1)
	if !ok || cardIdx != 0 || matchIdx != 1 {
		t.Errorf("findMatch(1) = (%d, %d, %v), want (0, 1, true)", cardIdx, matchIdx, ok)
	}
}

func TestFindMatchCrossesIntoNextCard(t *testing.T) {
	app := &App{searchResults: []model.FileResult{
		{Matches: make([]model.ContentMatch, 2)},
		{Matches: make([]model.ContentMatch, 2)},
	}}
	v := &resultsView{app: app, order: []int{0, 1}, selIdx: 0, curMatch: 1} // last match of card 0

	cardIdx, matchIdx, ok := v.findMatch(1)
	if !ok || cardIdx != 1 || matchIdx != 0 {
		t.Errorf("findMatch(1) crossing into next card = (%d, %d, %v), want (1, 0, true)", cardIdx, matchIdx, ok)
	}

	// And the reverse direction, from the first match of card 1.
	v.selIdx, v.curMatch = 1, 0
	cardIdx, matchIdx, ok = v.findMatch(-1)
	if !ok || cardIdx != 0 || matchIdx != 1 {
		t.Errorf("findMatch(-1) crossing into previous card = (%d, %d, %v), want (0, 1, true)", cardIdx, matchIdx, ok)
	}
}

// TestFindMatchSkipsCardWithNoMatches guards the filename-only-hit case: a
// card with zero content matches has nothing for the inner buttons to land
// on, so stepping must skip straight past it to the next card that does.
func TestFindMatchSkipsCardWithNoMatches(t *testing.T) {
	app := &App{searchResults: []model.FileResult{
		{Matches: make([]model.ContentMatch, 1)},
		{Matches: nil},
		{Matches: make([]model.ContentMatch, 1)},
	}}
	v := &resultsView{app: app, order: []int{0, 1, 2}, selIdx: 0, curMatch: 0}

	cardIdx, matchIdx, ok := v.findMatch(1)
	if !ok || cardIdx != 2 || matchIdx != 0 {
		t.Errorf("findMatch(1) skipping an empty card = (%d, %d, %v), want (2, 0, true)", cardIdx, matchIdx, ok)
	}
}

func TestFindMatchReturnsFalseAtBoundary(t *testing.T) {
	app := &App{searchResults: []model.FileResult{{Matches: make([]model.ContentMatch, 1)}}}
	v := &resultsView{app: app, order: []int{0}, selIdx: 0, curMatch: 0}

	if _, _, ok := v.findMatch(1); ok {
		t.Error("findMatch(1) at the last match overall, want ok=false")
	}
	if _, _, ok := v.findMatch(-1); ok {
		t.Error("findMatch(-1) at the first match overall, want ok=false")
	}
}

func TestResetCurMatch(t *testing.T) {
	app := &App{searchResults: []model.FileResult{
		{Matches: make([]model.ContentMatch, 2)},
		{Matches: nil},
	}}
	v := &resultsView{app: app, order: []int{0, 1}}

	v.selIdx = 0
	v.resetCurMatch()
	if v.curMatch != 0 {
		t.Errorf("resetCurMatch on a card with matches: curMatch = %d, want 0", v.curMatch)
	}

	v.selIdx = 1
	v.resetCurMatch()
	if v.curMatch != -1 {
		t.Errorf("resetCurMatch on a card with no matches: curMatch = %d, want -1", v.curMatch)
	}
}
