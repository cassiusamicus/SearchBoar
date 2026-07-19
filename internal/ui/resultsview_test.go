package ui

import (
	"regexp"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

func TestHighlightSegmentsBoldsEveryOccurrence(t *testing.T) {
	re := regexp.MustCompile(`(?i)target`)
	segs := highlightSegments("target one, another target here", re)

	var bold []string
	for _, s := range segs {
		ts, ok := s.(*widget.TextSegment)
		if !ok {
			continue
		}
		if ts.Style.TextStyle.Bold {
			bold = append(bold, ts.Text)
		}
	}
	if len(bold) != 2 || bold[0] != "target" || bold[1] != "target" {
		t.Errorf("bold segments = %v, want [\"target\" \"target\"]", bold)
	}
}

func TestHighlightSegmentsNoMatchStaysPlain(t *testing.T) {
	re := regexp.MustCompile(`target`)
	segs := highlightSegments("nothing to see here", re)
	if len(segs) != 1 {
		t.Fatalf("got %d segments, want 1 (no match -> whole line as one plain segment)", len(segs))
	}
	ts, ok := segs[0].(*widget.TextSegment)
	if !ok || ts.Style.TextStyle.Bold {
		t.Errorf("segment = %+v, want a single non-bold segment", segs[0])
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

	block := v.buildMatchBlock(m, re)
	vbox, ok := block.(*fyne.Container)
	if !ok {
		t.Fatalf("buildMatchBlock returned %T, want *fyne.Container", block)
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

	boldCountInRow := func(row *widget.RichText) int {
		n := 0
		for _, s := range row.Segments {
			if ts, ok := s.(*widget.TextSegment); ok && ts.Style.TextStyle.Bold {
				n++
			}
		}
		return n
	}

	if boldCountInRow(rows[0]) != 0 {
		t.Errorf("row 0 (no match) has bold segments, want none")
	}
	if boldCountInRow(rows[1]) != 1 {
		t.Errorf("row 1 (LineNum, has a real match) bold count = %d, want 1", boldCountInRow(rows[1]))
	}
	if boldCountInRow(rows[2]) != 1 {
		t.Errorf("row 2 (not LineNum, but also a real match) bold count = %d, want 1 -- this is the bug this test guards against", boldCountInRow(rows[2]))
	}
}
