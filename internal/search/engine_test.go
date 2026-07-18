package search

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDocx(t *testing.T, path string, paragraphs ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://example.com/w"><w:body>`
	for _, p := range paragraphs {
		xml += `<w:p><w:r><w:t>` + p + `</w:t></w:r></w:p>`
	}
	xml += `</w:body></w:document>`
	if _, err := w.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// buildTree lays out a fixed set of files under dir exercising every filter
// dimension the engine supports: recursion, hidden files, exclude globs,
// size filters, binary detection, and docx extraction.
func buildTree(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\nfoo bar\n")
	writeFile(t, filepath.Join(dir, "b.md"), "epicurus wrote about pleasure\n")
	writeFile(t, filepath.Join(dir, "sub", "c.txt"), "nested file with target\nsecond line\nthird line\n")
	writeFile(t, filepath.Join(dir, ".hidden.txt"), "hidden target\n")
	writeFile(t, filepath.Join(dir, "excluded.pyc"), "target inside excluded\n")
	writeFile(t, filepath.Join(dir, "binary.dat"), "bin\x00ary target\n")
	writeFile(t, filepath.Join(dir, "big.txt"), "target padding line\n"+string(make([]byte, 2000)))
	writeDocx(t, filepath.Join(dir, "notes.docx"), "intro paragraph", "this has a target word", "closing paragraph")
}

func runCollect(t *testing.T, e *Engine, opts Options) []model.FileResult {
	t.Helper()
	results := make(chan model.FileResult)
	progress := make(chan Progress, 100)
	var got []model.FileResult
	done := make(chan error, 1)

	go func() {
		done <- e.Run(context.Background(), opts, results, progress)
	}()

	resultsDone := false
	progressDone := false
	for !resultsDone || !progressDone {
		select {
		case r, ok := <-results:
			if !ok {
				resultsDone = true
				continue
			}
			got = append(got, r)
		case _, ok := <-progress:
			if !ok {
				progressDone = true
			}
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	return got
}

func names(results []model.FileResult) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

func TestFilenameOnlySearch(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}
	got := runCollect(t, e, Options{
		Dir:         dir,
		FilePattern: `\.txt$`,
		Recursive:   true,
	})

	want := []string{"a.txt", "big.txt", "c.txt"} // hidden + binary.dat's ".dat" excluded by pattern
	if got2 := names(got); !equalStrings(got2, want) {
		t.Errorf("got %v, want %v", got2, want)
	}
}

func TestContentSearchWalkFallback(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2} // no RipgrepPath -> forces walk path
	got := runCollect(t, e, Options{
		Dir:            dir,
		FilePattern:    ".*",
		ContentEnabled: true,
		ContentPattern: "target",
		Recursive:      true,
		ContextBefore:  1,
		ContextAfter:   1,
	})

	// Expect: c.txt (text), notes.docx (docx extraction) match.
	// binary.dat is skipped (binary), .hidden.txt skipped (hidden default off),
	// excluded.pyc has no exclude glob set here so it should match too.
	want := []string{"c.txt", "excluded.pyc", "notes.docx"}
	if got2 := names(got); !equalStrings(got2, want) {
		t.Errorf("got %v, want %v", got2, want)
	}

	for _, r := range got {
		if r.Name == "c.txt" {
			if len(r.Matches) != 1 {
				t.Fatalf("expected 1 match in c.txt, got %d", len(r.Matches))
			}
			m := r.Matches[0]
			if m.LineNum != 1 || m.ContextStartLine != 1 {
				t.Errorf("c.txt match = %+v, unexpected line numbers", m)
			}
			if len(m.ContextLines) != 2 { // before=1 clamped at start, after=1
				t.Errorf("c.txt context lines = %v, want 2", m.ContextLines)
			}
		}
		if r.Name == "notes.docx" {
			if len(r.Matches) != 1 {
				t.Fatalf("expected 1 match in notes.docx, got %d", len(r.Matches))
			}
		}
	}
}

func TestContentSearchRipgrepMatchesWalkFallback(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	dir := t.TempDir()
	buildTree(t, dir)

	opts := Options{
		Dir:            dir,
		FilePattern:    `\.txt$`,
		ContentEnabled: true,
		ContentPattern: "target",
		Recursive:      true,
	}

	withRg := NewEngine()
	gotRg := runCollect(t, withRg, opts)

	withoutRg := &Engine{MaxWorkers: 2}
	gotWalk := runCollect(t, withoutRg, opts)

	if !equalStrings(names(gotRg), names(gotWalk)) {
		t.Errorf("ripgrep path found %v, walk path found %v", names(gotRg), names(gotWalk))
	}
}

func TestExcludeGlobs(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}
	got := runCollect(t, e, Options{
		Dir:            dir,
		FilePattern:    ".*",
		ContentEnabled: true,
		ContentPattern: "target",
		Recursive:      true,
		ExcludeGlobs:   []string{"*.pyc"},
	})

	for _, r := range got {
		if r.Name == "excluded.pyc" {
			t.Errorf("excluded.pyc should have been filtered out by ExcludeGlobs")
		}
	}
}

func TestHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}

	gotDefault := runCollect(t, e, Options{Dir: dir, FilePattern: `hidden`, Recursive: true})
	if len(gotDefault) != 0 {
		t.Errorf("expected hidden file excluded by default, got %v", names(gotDefault))
	}

	gotIncluded := runCollect(t, e, Options{Dir: dir, FilePattern: `hidden`, Recursive: true, IncludeHidden: true})
	if len(gotIncluded) != 1 {
		t.Errorf("expected hidden file included, got %v", names(gotIncluded))
	}
}

func TestNonRecursiveSkipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}
	got := runCollect(t, e, Options{Dir: dir, FilePattern: `\.txt$`, Recursive: false})
	for _, r := range got {
		if r.Name == "c.txt" {
			t.Errorf("non-recursive search should not have descended into sub/")
		}
	}
}

func TestSizeFilters(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}
	got := runCollect(t, e, Options{Dir: dir, FilePattern: `\.txt$`, Recursive: true, MinSizeBytes: 1000})
	want := []string{"big.txt"}
	if got2 := names(got); !equalStrings(got2, want) {
		t.Errorf("got %v, want %v", got2, want)
	}
}

func TestCaseSensitivity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.txt"), "Target\n")

	e := &Engine{MaxWorkers: 2}
	gotSensitive := runCollect(t, e, Options{
		Dir: dir, FilePattern: ".*", ContentEnabled: true, ContentPattern: "target",
		Recursive: true, CaseSensitive: true,
	})
	if len(gotSensitive) != 0 {
		t.Errorf("case-sensitive search should not match 'Target' against 'target', got %v", names(gotSensitive))
	}

	gotInsensitive := runCollect(t, e, Options{
		Dir: dir, FilePattern: ".*", ContentEnabled: true, ContentPattern: "target",
		Recursive: true, CaseSensitive: false,
	})
	if len(gotInsensitive) != 1 {
		t.Errorf("case-insensitive search should match, got %v", names(gotInsensitive))
	}
}

func TestCancellation(t *testing.T) {
	dir := t.TempDir()
	buildTree(t, dir)

	e := &Engine{MaxWorkers: 2}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := make(chan model.FileResult)
	go func() {
		for range results {
		}
	}()
	err := e.Run(ctx, Options{Dir: dir, FilePattern: ".*", Recursive: true}, results, nil)
	if err == nil {
		t.Error("expected an error from a pre-canceled context")
	}
}

func TestPDFExtractionErrorsOnNonPDF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.pdf")
	writeFile(t, path, "not actually a pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := extractPDFText(ctx, "", path); err == nil {
		t.Error("expected an error extracting text from a non-PDF file")
	}
}

func TestDocxExtraction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.docx")
	writeDocx(t, path, "first paragraph", "second has TARGETWORD inside")

	text, err := extractDOCXText(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "TARGETWORD") {
		t.Errorf("extracted docx text missing expected content: %q", text)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
