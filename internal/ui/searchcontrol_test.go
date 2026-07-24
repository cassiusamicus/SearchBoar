package ui

import (
	"regexp"
	"testing"
	"time"

	"fyne.io/fyne/v2/widget"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

func TestRemoveResultByPathRemovesOnlyTheMatchingEntry(t *testing.T) {
	results := []model.FileResult{
		{FileEntry: model.FileEntry{Path: "/a.txt"}},
		{FileEntry: model.FileEntry{Path: "/b.txt"}},
		{FileEntry: model.FileEntry{Path: "/c.txt"}},
	}

	got := removeResultByPath(results, "/b.txt")
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].Path != "/a.txt" || got[1].Path != "/c.txt" {
		t.Errorf("got %+v, want [/a.txt /c.txt] (order preserved, /b.txt gone)", got)
	}
}

func TestRemoveResultByPathNoMatchIsNoOp(t *testing.T) {
	results := []model.FileResult{{FileEntry: model.FileEntry{Path: "/a.txt"}}}
	got := removeResultByPath(results, "/not-there.txt")
	if len(got) != 1 || got[0].Path != "/a.txt" {
		t.Errorf("got %+v, want the input unchanged", got)
	}
}

// newTestStartTabForOptions builds just enough of a startTab's widgets for
// searchOptionsTemplate to read from, without running build() (which needs
// a real app/window) -- the fields it doesn't touch are left at their zero
// value.
func newTestStartTabForOptions() *startTab {
	return &startTab{
		fileEntry:          widget.NewEntry(),
		contentCombo:       widget.NewSelectEntry(nil),
		recursiveCheck:     widget.NewCheck("", nil),
		caseCheck:          widget.NewCheck("", nil),
		excludeHiddenCheck: widget.NewCheck("", nil),
		excludeTildeCheck:  widget.NewCheck("", nil),
		beforeSpin:         newIntSpinner(0, 10, 0),
		afterSpin:          newIntSpinner(0, 10, 0),
		minSizeEntry:       widget.NewEntry(),
		maxSizeEntry:       widget.NewEntry(),
		excludeEntry:       widget.NewEntry(),
	}
}

// TestSearchOptionsTemplateIncludesHiddenAndTildeFilesByDefault guards the
// reversed default: excludeHiddenCheck/excludeTildeCheck start unchecked,
// so a fresh search includes hidden and ~ backup files unless the user
// opts into excluding them -- the opposite of this app's old
// "Include hidden files" default.
func TestSearchOptionsTemplateIncludesHiddenAndTildeFilesByDefault(t *testing.T) {
	a := &App{start: newTestStartTabForOptions()}
	opts := a.searchOptionsTemplate()
	if !opts.IncludeHidden {
		t.Error("IncludeHidden = false, want true (hidden files included by default)")
	}
	for _, g := range opts.ExcludeGlobs {
		if g == tildeBackupGlob {
			t.Errorf("ExcludeGlobs = %v, want no %q entry when excludeTildeCheck is unchecked", opts.ExcludeGlobs, tildeBackupGlob)
		}
	}
}

// TestSearchOptionsTemplateExcludesHiddenAndTildeFilesWhenChecked guards
// the opt-in direction: checking either box should actually exclude that
// category, not just flip a field nothing reads.
func TestSearchOptionsTemplateExcludesHiddenAndTildeFilesWhenChecked(t *testing.T) {
	start := newTestStartTabForOptions()
	// Set the field directly, not SetChecked -- SetChecked triggers a
	// widget Refresh, which needs a running Fyne app/window context this
	// standalone unit test doesn't have.
	start.excludeHiddenCheck.Checked = true
	start.excludeTildeCheck.Checked = true
	a := &App{start: start}

	opts := a.searchOptionsTemplate()
	if opts.IncludeHidden {
		t.Error("IncludeHidden = true, want false when excludeHiddenCheck is checked")
	}
	found := false
	for _, g := range opts.ExcludeGlobs {
		if g == tildeBackupGlob {
			found = true
		}
	}
	if !found {
		t.Errorf("ExcludeGlobs = %v, want it to include %q when excludeTildeCheck is checked", opts.ExcludeGlobs, tildeBackupGlob)
	}
}

func TestGroupTermMatchesGroupsConsecutiveRowsByPath(t *testing.T) {
	now := time.Now()
	matches := []cache.TermMatch{
		{Path: "/a.txt", ModTime: now, Size: 10, LineNum: 2, ContextStartLine: 2, ContextLines: []string{"one"}},
		{Path: "/a.txt", ModTime: now, Size: 10, LineNum: 9, ContextStartLine: 9, ContextLines: []string{"two"}},
		{Path: "/b.txt", ModTime: now, Size: 20, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"three"}},
	}

	got := groupTermMatches(matches, nil)
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].Path != "/a.txt" || len(got[0].Matches) != 2 {
		t.Errorf("got[0] = %+v, want /a.txt with 2 matches", got[0])
	}
	if got[1].Path != "/b.txt" || len(got[1].Matches) != 1 {
		t.Errorf("got[1] = %+v, want /b.txt with 1 match", got[1])
	}
	if got[0].Name != "a.txt" {
		t.Errorf("got[0].Name = %q, want \"a.txt\"", got[0].Name)
	}
}

func TestGroupTermMatchesFiltersByFilenameRegex(t *testing.T) {
	now := time.Now()
	matches := []cache.TermMatch{
		{Path: "/a.pdf", ModTime: now, Size: 10, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"x"}},
		{Path: "/b.txt", ModTime: now, Size: 20, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"y"}},
	}

	re := regexp.MustCompile(`\.txt$`)
	got := groupTermMatches(matches, re)
	if len(got) != 1 || got[0].Path != "/b.txt" {
		t.Fatalf("got %+v, want only /b.txt", got)
	}
}

func TestGroupTermMatchesNilRegexKeepsEverything(t *testing.T) {
	now := time.Now()
	matches := []cache.TermMatch{
		{Path: "/a.pdf", ModTime: now, Size: 10, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"x"}},
	}
	got := groupTermMatches(matches, nil)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
}

func TestGroupTermMatchesEmptyInput(t *testing.T) {
	if got := groupTermMatches(nil, nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
