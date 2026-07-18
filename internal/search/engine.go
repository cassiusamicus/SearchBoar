// Package search implements SearchBoar's local file search: a filename
// regex, an optional content regex searched two ways (ripgrep when
// available, otherwise a worker-pool walk with PDF/DOCX extraction), and
// context-window match reporting. It has no GUI dependency and is meant to
// be driven by internal/ui or a CLI.
package search

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/cassiusamicus/Utilities/internal/cache"
	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

// Engine runs searches. Its zero value is usable but won't have ripgrep or
// pdftotext detected; use NewEngine to auto-detect both.
type Engine struct {
	RipgrepPath   string // "" if ripgrep isn't on PATH
	PdftotextPath string // "" if pdftotext isn't on PATH
	MaxWorkers    int

	// Cache, if set, is consulted before extracting a candidate file's text
	// (a plain read for text files, real extraction for PDF/DOCX) and
	// updated after -- so a repeat search over the same tree only pays the
	// extraction cost once per unchanged file. A nil Cache disables this
	// (every Cache method is a nil-safe no-op), and it's never consulted on
	// the ripgrep fast path, which already reads files itself.
	Cache *cache.Cache
}

// NewEngine builds an Engine with ripgrep/pdftotext auto-detected via PATH
// and a worker count capped at 8, matching the original app's
// min(8, cpu_count) sizing for its ThreadPoolExecutor.
func NewEngine() *Engine {
	rg, _ := exec.LookPath("rg")
	pdftotext, _ := exec.LookPath("pdftotext")
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}
	return &Engine{RipgrepPath: rg, PdftotextPath: pdftotext, MaxWorkers: workers}
}

// Run executes a search and streams results/progress on the given
// channels, closing both when done (whether it finishes, errors, or ctx is
// canceled). It blocks until the search is complete.
func (e *Engine) Run(ctx context.Context, opts Options, results chan<- model.FileResult, progress chan<- Progress) error {
	defer close(results)
	if progress != nil {
		defer close(progress)
	}

	filenameRe, err := compileRegex(opts.FilePattern, opts.CaseSensitive)
	if err != nil {
		return fmt.Errorf("invalid file pattern: %w", err)
	}

	var contentRe *regexp.Regexp
	if opts.ContentEnabled && strings.TrimSpace(opts.ContentPattern) != "" {
		contentRe, err = compileRegex(opts.ContentPattern, opts.CaseSensitive)
		if err != nil {
			return fmt.Errorf("invalid content pattern: %w", err)
		}
	}

	canUseRipgrep := e.RipgrepPath != "" &&
		!opts.DisableRipgrep &&
		contentRe != nil &&
		opts.Recursive &&
		!patternMentionsExt(opts.FilePattern, ".pdf") &&
		!patternMentionsExt(opts.FilePattern, ".docx")

	if canUseRipgrep {
		if err := e.runRipgrep(ctx, opts, filenameRe, contentRe, results, progress); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
		// Any non-cancellation ripgrep failure (timeout, crash, missing
		// binary race) falls through to the walk-based search below,
		// matching the original app's silent fallback behavior.
	}

	return e.runWalk(ctx, opts, filenameRe, contentRe, results, progress)
}

// CompileRegex is the exported form of the engine's own pattern compilation
// (default-to-".*", optional case-insensitivity), so callers (e.g. the UI's
// match-highlighting) can build the identical regex the engine used.
func CompileRegex(pattern string, caseSensitive bool) (*regexp.Regexp, error) {
	return compileRegex(pattern, caseSensitive)
}

func compileRegex(pattern string, caseSensitive bool) (*regexp.Regexp, error) {
	if pattern == "" {
		pattern = ".*"
	}
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

// patternMentionsExt is a heuristic substring check (not real regex
// analysis) used only to decide whether ripgrep should be bypassed in favor
// of the PDF/DOCX-aware walk path, exactly as the original app did.
func patternMentionsExt(pattern, ext string) bool {
	return strings.Contains(strings.ToLower(pattern), strings.ToLower(ext))
}

func (e *Engine) runWalk(ctx context.Context, opts Options, filenameRe, contentRe *regexp.Regexp, results chan<- model.FileResult, progress chan<- Progress) error {
	candidates, err := collectCandidates(ctx, opts, filenameRe)
	if err != nil {
		return err
	}
	total := len(candidates)

	numWorkers := e.MaxWorkers
	if numWorkers < 1 {
		numWorkers = 1
	}

	jobs := make(chan candidate)
	var wg sync.WaitGroup
	var searched, found int64
	var progressMu sync.Mutex
	var lastProgress time.Time

	sendProgress := func(force bool) {
		if progress == nil {
			return
		}
		progressMu.Lock()
		due := force || time.Since(lastProgress) >= 200*time.Millisecond
		if due {
			lastProgress = time.Now()
		}
		progressMu.Unlock()
		if !due {
			return
		}
		select {
		case progress <- Progress{Searched: int(atomic.LoadInt64(&searched)), Found: int(atomic.LoadInt64(&found)), Total: total}:
		default:
		}
	}

	worker := func() {
		defer wg.Done()
		for c := range jobs {
			res, matched := e.searchFile(ctx, c, opts, contentRe)
			atomic.AddInt64(&searched, 1)
			if matched {
				atomic.AddInt64(&found, 1)
				select {
				case results <- res:
				case <-ctx.Done():
				}
			}
			sendProgress(false)
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

produceLoop:
	for _, c := range candidates {
		select {
		case <-ctx.Done():
			break produceLoop
		case jobs <- c:
		}
	}
	close(jobs)
	wg.Wait()
	sendProgress(true)

	return ctx.Err()
}

func (e *Engine) searchFile(ctx context.Context, c candidate, opts Options, contentRe *regexp.Regexp) (model.FileResult, bool) {
	entry := model.FileEntry{Path: c.path, Name: c.info.Name(), ModTime: c.info.ModTime(), Size: c.info.Size()}
	res := model.FileResult{FileEntry: entry}

	if contentRe == nil {
		return res, true // filename-only search; already passed the filename regex
	}

	ext := strings.ToLower(filepath.Ext(c.path))

	var extract func() (string, error)
	switch ext {
	case ".pdf":
		extract = func() (string, error) { return extractPDFText(ctx, e.PdftotextPath, c.path) }
	case ".docx":
		extract = func() (string, error) { return extractDOCXText(c.path) }
	default:
		isBin, err := isBinaryFile(c.path)
		if err != nil || isBin {
			return res, false
		}
		extract = func() (string, error) { return readText(c.path) }
	}

	text, err := e.extractText(c, extract)
	if err != nil {
		return res, false
	}

	res.Matches = matchLinesInSlice(splitLines(text), contentRe, opts.ContextBefore, opts.ContextAfter)
	return res, len(res.Matches) > 0
}

// extractText returns c's text content, from the cache if a fresh entry
// exists (same path, mtime, and size), otherwise via extract -- which it
// then stores for next time.
func (e *Engine) extractText(c candidate, extract func() (string, error)) (string, error) {
	if text, ok := e.Cache.GetText(c.path, c.info.ModTime(), c.info.Size()); ok {
		return text, nil
	}
	text, err := extract()
	if err != nil {
		return "", err
	}
	e.Cache.PutText(c.path, c.info.ModTime(), c.info.Size(), text)
	return text, nil
}
