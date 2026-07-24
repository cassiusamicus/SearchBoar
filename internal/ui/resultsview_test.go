package ui

import (
	"regexp"
	"strings"
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

func TestTrimContextLinesUnlimitedReturnsAllLines(t *testing.T) {
	m := model.ContentMatch{LineNum: 5, ContextStartLine: 3, ContextLines: []string{"a", "b", "c"}}
	lines, start := trimContextLines(m, 0)
	if len(lines) != 3 || start != 3 {
		t.Errorf("trimContextLines(m, 0) = (%v, %d), want the lines unchanged", lines, start)
	}
}

func TestTrimContextLinesShorterThanMaxReturnsAllLines(t *testing.T) {
	m := model.ContentMatch{LineNum: 3, ContextStartLine: 3, ContextLines: []string{"a", "b"}}
	lines, start := trimContextLines(m, 5)
	if len(lines) != 2 || start != 3 {
		t.Errorf("trimContextLines under the cap = (%v, %d), want the lines unchanged", lines, start)
	}
}

// TestTrimContextLinesCentersOnMatchLine guards the reason this centers on
// m.LineNum instead of always keeping the first maxLines from the top of
// the block: a match recorded near the end of a merged context block would
// otherwise have its own line trimmed away entirely.
func TestTrimContextLinesCentersOnMatchLine(t *testing.T) {
	m := model.ContentMatch{
		LineNum:          9,
		ContextStartLine: 5,
		ContextLines:     []string{"l5", "l6", "l7", "l8", "l9", "l10", "l11"}, // match is "l9", index 4
	}
	lines, start := trimContextLines(m, 3)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	matchLineIdx := m.LineNum - start
	if matchLineIdx < 0 || matchLineIdx >= len(lines) || lines[matchLineIdx] != "l9" {
		t.Errorf("trimContextLines(m, 3) = (%v, start=%d), want the match's own line (\"l9\") kept in the window", lines, start)
	}
}

// TestTrimContextLinesNearBlockEdgeStaysInBounds guards the boundary case:
// centering on a match near either edge of the block must not walk the
// window past the actual data on the other side.
func TestTrimContextLinesNearBlockEdgeStaysInBounds(t *testing.T) {
	m := model.ContentMatch{
		LineNum:          1,
		ContextStartLine: 1,
		ContextLines:     []string{"l1", "l2", "l3", "l4", "l5"}, // match is the very first line
	}
	lines, start := trimContextLines(m, 3)
	if len(lines) != 3 || start != 1 || lines[0] != "l1" {
		t.Errorf("trimContextLines with the match at the block's start = (%v, start=%d), want [\"l1\" \"l2\" \"l3\"] at start=1", lines, start)
	}
}

func TestTruncateListRowFitsWithoutTruncation(t *testing.T) {
	got := truncateListRow("a.go", "/repo   •   2026-01-01   •   1 KB", 70)
	want := "a.go   /repo   •   2026-01-01   •   1 KB"
	if got != want {
		t.Errorf("truncateListRow (fits) = %q, want %q", got, want)
	}
}

// TestTruncateListRowTruncatesDetailNotName guards the point of this
// function: the name is the primary identifier and must never be cut, even
// when the combined line is far too long to fit -- only the detail suffix
// (path/date/size) absorbs the truncation.
func TestTruncateListRowTruncatesDetailNotName(t *testing.T) {
	name := "alpha.go"
	detail := "/very/long/deeply/nested/path/that/does/not/fit   •   2026-01-01   •   4.9 KB"
	got := truncateListRow(name, detail, 40)

	if !strings.HasPrefix(got, name) {
		t.Fatalf("truncateListRow = %q, want it to start with the untouched name %q", got, name)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateListRow = %q, want a trailing ellipsis marking the truncation", got)
	}
	if length := len([]rune(got)); length > 40 {
		t.Errorf("truncateListRow result is %d runes, want <= 40", length)
	}
}

func TestTruncateListRowExtremelyLongNameAloneReturnsJustName(t *testing.T) {
	name := strings.Repeat("x", 100)
	got := truncateListRow(name, "/repo   •   2026-01-01   •   1 KB", 40)
	if got != name {
		t.Errorf("truncateListRow with no budget left for detail = %q, want just the name %q", got, name)
	}
}

func TestMatchesForDisplayReturnsEveryMatchByDefault(t *testing.T) {
	v := &resultsView{}
	res := model.FileResult{Matches: make([]model.ContentMatch, 3)}
	if got := v.matchesForDisplay(res); len(got) != 3 {
		t.Errorf("matchesForDisplay with showFirstMatchOnly=false = %d matches, want 3", len(got))
	}
}

func TestMatchesForDisplayCapsToFirstWhenEnabled(t *testing.T) {
	v := &resultsView{showFirstMatchOnly: true}
	res := model.FileResult{Matches: []model.ContentMatch{{LineNum: 1}, {LineNum: 2}, {LineNum: 3}}}
	got := v.matchesForDisplay(res)
	if len(got) != 1 || got[0].LineNum != 1 {
		t.Errorf("matchesForDisplay with showFirstMatchOnly=true = %+v, want just the first match", got)
	}
}

// TestMatchesForDisplayNoMatchesStaysEmpty guards the filename-only-hit
// case: capping to "the first match" must not turn zero matches into a
// panic or a phantom entry.
func TestMatchesForDisplayNoMatchesStaysEmpty(t *testing.T) {
	v := &resultsView{showFirstMatchOnly: true}
	res := model.FileResult{}
	if got := v.matchesForDisplay(res); len(got) != 0 {
		t.Errorf("matchesForDisplay on a filename-only hit = %+v, want empty", got)
	}
}

// TestMatchesOfRespectsShowFirstMatchOnly guards the actual point of
// routing matchesOf through matchesForDisplay: the inner Back/Forward
// stepper (findMatch) must see the same capped count buildCard renders,
// or stepping could try to land on a match block that was never built.
func TestMatchesOfRespectsShowFirstMatchOnly(t *testing.T) {
	app := &App{searchResults: []model.FileResult{
		{Matches: make([]model.ContentMatch, 3)},
	}}
	v := &resultsView{app: app, order: []int{0}, showFirstMatchOnly: true}
	if got := v.matchesOf(0); len(got) != 1 {
		t.Errorf("matchesOf with showFirstMatchOnly=true = %d matches, want 1", len(got))
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
