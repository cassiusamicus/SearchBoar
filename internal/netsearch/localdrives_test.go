package netsearch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchLocalWithExplicitRoots(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "x")
	mustWrite(t, filepath.Join(dir, "sub", "b.txt"), "x")
	mustWrite(t, filepath.Join(dir, "sub", "c.md"), "x")
	mustWrite(t, filepath.Join(dir, "excluded", "d.txt"), "x")

	e := NewEngine()
	opts := Options{
		Pattern:     "*.txt",
		LocalRoots:  []string{dir},
		ExcludeDirs: []string{filepath.Join(dir, "excluded")},
		SearchLocal: true,
	}

	results := make(chan Result)
	var got []Result
	done := make(chan error, 1)
	go func() { done <- e.Run(context.Background(), opts, results, nil) }()
	for r := range results {
		got = append(got, r)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 matches (a.txt, sub/b.txt), got %d: %+v", len(got), got)
	}
	for _, r := range got {
		if r.NetworkPath == "file://"+filepath.Join(dir, "excluded", "d.txt") {
			t.Error("excluded directory should not have been searched")
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
