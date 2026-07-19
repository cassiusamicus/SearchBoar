package ui

import (
	"regexp"
	"testing"
	"time"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
)

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
