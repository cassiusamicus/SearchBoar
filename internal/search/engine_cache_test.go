package search

import (
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
)

func openTestCache(t *testing.T) *cache.Cache {
	t.Helper()
	c, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCachePopulatedAfterContentSearch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\ntarget line\n")

	c := openTestCache(t)
	e := &Engine{MaxWorkers: 2, Cache: c}
	opts := Options{Dir: dir, FilePattern: `\.txt$`, ContentPattern: "target", ContentEnabled: true, Recursive: true}

	results := runCollect(t, e, opts)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	rows, _, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if rows != 1 {
		t.Fatalf("cache rows = %d, want 1", rows)
	}

	text, ok := c.GetText(filepath.Join(dir, "a.txt"), results[0].ModTime, results[0].Size)
	if !ok {
		t.Fatal("expected cache hit for the searched file")
	}
	if text != "hello world\ntarget line\n" {
		t.Fatalf("cached text = %q", text)
	}
}

func TestCachedSearchStillFindsUpdatedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	writeFile(t, path, "first version\n")

	c := openTestCache(t)
	e := &Engine{MaxWorkers: 2, Cache: c}
	opts := Options{Dir: dir, FilePattern: `\.txt$`, ContentPattern: "second", ContentEnabled: true, Recursive: true}

	// First search: "second" isn't in the file yet.
	if got := runCollect(t, e, opts); len(got) != 0 {
		t.Fatalf("first search: got %d results, want 0", len(got))
	}

	// Modify the file -- mtime must move forward far enough that a coarse
	// filesystem timestamp clock actually registers the change.
	time.Sleep(10 * time.Millisecond)
	writeFile(t, path, "second version\n")

	got := runCollect(t, e, opts)
	if len(got) != 1 {
		t.Fatalf("second search: got %d results, want 1 (cache must not serve stale text after the file changed)", len(got))
	}
}

func TestNilCacheDoesNotBreakContentSearch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "target line\n")

	e := &Engine{MaxWorkers: 2} // Cache left nil
	opts := Options{Dir: dir, FilePattern: `\.txt$`, ContentPattern: "target", ContentEnabled: true, Recursive: true}

	got := runCollect(t, e, opts)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
}
