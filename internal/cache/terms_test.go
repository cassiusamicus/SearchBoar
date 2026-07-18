package cache

import (
	"testing"
	"time"
)

func TestAddTermThenListShowsIt(t *testing.T) {
	c := openTest(t)
	if err := c.AddTerm("epicurus"); err != nil {
		t.Fatalf("AddTerm: %v", err)
	}

	terms, err := c.ListTerms()
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("got %d terms, want 1", len(terms))
	}
	if terms[0].Term != "epicurus" {
		t.Fatalf("term = %q, want %q", terms[0].Term, "epicurus")
	}
	if !terms[0].LastIndexedAt.IsZero() {
		t.Fatal("expected zero LastIndexedAt before any indexing")
	}
	if terms[0].MatchCount != 0 {
		t.Fatalf("MatchCount = %d, want 0", terms[0].MatchCount)
	}
}

func TestAddTermTwiceIsIdempotent(t *testing.T) {
	c := openTest(t)
	c.AddTerm("epicurus")
	c.AddTerm("epicurus")

	terms, err := c.ListTerms()
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("got %d terms, want 1", len(terms))
	}
}

func TestSaveTermMatchesRoundTripsAndStampsIndexTime(t *testing.T) {
	c := openTest(t)
	c.AddTerm("pleasure")

	mtime := time.Now().Truncate(time.Second)
	matches := []TermMatch{
		{Path: "/a/b.txt", ModTime: mtime, Size: 10, LineNum: 3, ContextStartLine: 2, ContextLines: []string{"one", "pleasure here", "three"}},
		{Path: "/c/d.txt", ModTime: mtime, Size: 20, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"pleasure again"}},
	}
	if err := c.SaveTermMatches("pleasure", matches); err != nil {
		t.Fatalf("SaveTermMatches: %v", err)
	}

	got, err := c.GetTermMatches("pleasure")
	if err != nil {
		t.Fatalf("GetTermMatches: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d matches, want 2", len(got))
	}
	if got[0].Path != "/a/b.txt" || len(got[0].ContextLines) != 3 || got[0].ContextLines[1] != "pleasure here" {
		t.Fatalf("match[0] = %+v", got[0])
	}

	terms, err := c.ListTerms()
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("got %d terms, want 1", len(terms))
	}
	if terms[0].MatchCount != 2 {
		t.Fatalf("MatchCount = %d, want 2", terms[0].MatchCount)
	}
	if terms[0].LastIndexedAt.IsZero() {
		t.Fatal("expected non-zero LastIndexedAt after SaveTermMatches")
	}
}

func TestSaveTermMatchesReplacesPreviousRun(t *testing.T) {
	c := openTest(t)
	c.AddTerm("pleasure")
	mtime := time.Now().Truncate(time.Second)

	c.SaveTermMatches("pleasure", []TermMatch{
		{Path: "/old.txt", ModTime: mtime, Size: 5, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"stale match"}},
	})
	c.SaveTermMatches("pleasure", []TermMatch{
		{Path: "/new.txt", ModTime: mtime, Size: 6, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"fresh match"}},
	})

	got, err := c.GetTermMatches("pleasure")
	if err != nil {
		t.Fatalf("GetTermMatches: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/new.txt" {
		t.Fatalf("got %+v, want only /new.txt", got)
	}
}

func TestSaveTermMatchesEmptyClearsStaleMatches(t *testing.T) {
	c := openTest(t)
	c.AddTerm("pleasure")
	mtime := time.Now().Truncate(time.Second)

	c.SaveTermMatches("pleasure", []TermMatch{
		{Path: "/old.txt", ModTime: mtime, Size: 5, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"stale match"}},
	})
	if err := c.SaveTermMatches("pleasure", nil); err != nil {
		t.Fatalf("SaveTermMatches(nil): %v", err)
	}

	got, err := c.GetTermMatches("pleasure")
	if err != nil {
		t.Fatalf("GetTermMatches: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d matches, want 0 after re-indexing with no hits", len(got))
	}
}

func TestRemoveTermDeletesTermAndMatches(t *testing.T) {
	c := openTest(t)
	c.AddTerm("pleasure")
	c.SaveTermMatches("pleasure", []TermMatch{
		{Path: "/a.txt", ModTime: time.Now(), Size: 1, LineNum: 1, ContextStartLine: 1, ContextLines: []string{"x"}},
	})

	if err := c.RemoveTerm("pleasure"); err != nil {
		t.Fatalf("RemoveTerm: %v", err)
	}

	terms, err := c.ListTerms()
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(terms) != 0 {
		t.Fatalf("got %d terms after RemoveTerm, want 0", len(terms))
	}
	matches, err := c.GetTermMatches("pleasure")
	if err != nil {
		t.Fatalf("GetTermMatches: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("got %d matches after RemoveTerm, want 0", len(matches))
	}
}

func TestNilCacheTermMethodsAreSafeNoOps(t *testing.T) {
	var c *Cache
	if err := c.AddTerm("x"); err != nil {
		t.Fatalf("AddTerm on nil cache: %v", err)
	}
	if err := c.RemoveTerm("x"); err != nil {
		t.Fatalf("RemoveTerm on nil cache: %v", err)
	}
	terms, err := c.ListTerms()
	if err != nil || terms != nil {
		t.Fatalf("ListTerms on nil cache = (%v, %v), want (nil, nil)", terms, err)
	}
	if err := c.SaveTermMatches("x", []TermMatch{{Path: "/a"}}); err != nil {
		t.Fatalf("SaveTermMatches on nil cache: %v", err)
	}
	matches, err := c.GetTermMatches("x")
	if err != nil || matches != nil {
		t.Fatalf("GetTermMatches on nil cache = (%v, %v), want (nil, nil)", matches, err)
	}
}
