package cache

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Cache {
	t.Helper()
	c, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestGetTextMissIsFalse(t *testing.T) {
	c := openTest(t)
	if _, ok := c.GetText("/no/such/file", time.Now(), 0); ok {
		t.Fatal("expected miss for never-cached path")
	}
}

func TestPutThenGetRoundTrips(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "hello world")

	text, ok := c.GetText("/a/b.txt", mtime, 42)
	if !ok {
		t.Fatal("expected hit after Put")
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want %q", text, "hello world")
	}
}

func TestGetTextStaleMtimeIsMiss(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "hello world")

	if _, ok := c.GetText("/a/b.txt", mtime.Add(time.Second), 42); ok {
		t.Fatal("expected miss when mtime differs")
	}
}

func TestGetTextStaleSizeIsMiss(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "hello world")

	if _, ok := c.GetText("/a/b.txt", mtime, 43); ok {
		t.Fatal("expected miss when size differs")
	}
}

func TestPutOverwritesExistingEntry(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "version one")
	c.PutText("/a/b.txt", mtime, 42, "version two")

	text, ok := c.GetText("/a/b.txt", mtime, 42)
	if !ok || text != "version two" {
		t.Fatalf("text = %q, ok = %v, want %q, true", text, ok, "version two")
	}
}

func TestClearRemovesAllEntries(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "hello")
	c.PutText("/c/d.txt", mtime, 7, "world")

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok := c.GetText("/a/b.txt", mtime, 42); ok {
		t.Fatal("expected miss after Clear")
	}
	rows, bytes, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if rows != 0 || bytes != 0 {
		t.Fatalf("Stats after Clear = (%d, %d), want (0, 0)", rows, bytes)
	}
}

func TestStatsCountsRowsAndBytes(t *testing.T) {
	c := openTest(t)
	mtime := time.Now().Truncate(time.Second)
	c.PutText("/a/b.txt", mtime, 42, "12345")
	c.PutText("/c/d.txt", mtime, 7, "1234567890")

	rows, bytes, err := c.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2", rows)
	}
	if bytes != 15 {
		t.Fatalf("bytes = %d, want 15", bytes)
	}
}

func TestNilCacheIsSafeNoOp(t *testing.T) {
	var c *Cache
	if _, ok := c.GetText("/a", time.Now(), 0); ok {
		t.Fatal("nil cache should always miss")
	}
	c.PutText("/a", time.Now(), 0, "text") // must not panic
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear on nil cache: %v", err)
	}
	rows, bytes, err := c.Stats()
	if err != nil || rows != 0 || bytes != 0 {
		t.Fatalf("Stats on nil cache = (%d, %d, %v), want (0, 0, nil)", rows, bytes, err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close on nil cache: %v", err)
	}
}
