package search

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codeberg.org/cassiusamicus/Utilities/internal/model"
)

type rgFileState struct {
	entry    model.FileEntry
	lastLine int
	cur      *model.ContentMatch
	matches  []model.ContentMatch
}

// runRipgrep runs `rg` over opts.Dir and streams grouped per-file results.
// ripgrep only filters by content pattern, case, and hidden-file rules, so
// the filename regex, exclude globs, and size filters are re-applied here
// against its output -- the same post-filtering the original app did.
//
// Consecutive matched lines are grouped into one model.ContentMatch per
// "cluster" using a gap-in-line-numbers heuristic (ripgrep's --no-heading
// output has no explicit separator between context blocks for different
// matches), matching the original app's simplified reimplementation of
// ripgrep's own `--` group separators.
func (e *Engine) runRipgrep(ctx context.Context, opts Options, filenameRe, contentRe *regexp.Regexp, results chan<- model.FileResult, progress chan<- Progress) error {
	args := []string{"--line-number", "--with-filename", "--no-heading"}
	if !opts.CaseSensitive {
		args = append(args, "--ignore-case")
	}
	if opts.ContextBefore > 0 {
		args = append(args, "--before-context", strconv.Itoa(opts.ContextBefore))
	}
	if opts.ContextAfter > 0 {
		args = append(args, "--after-context", strconv.Itoa(opts.ContextAfter))
	}
	if opts.IncludeHidden {
		args = append(args, "--hidden")
	}
	args = append(args, opts.ContentPattern, opts.Dir)

	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cctx, e.RipgrepPath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// rg exits 1 for "no matches found" -- not a failure.
			return nil
		}
		return runErr // any other failure triggers the walk-based fallback
	}

	states := map[string]*rgFileState{}
	var order []string
	gapThreshold := opts.ContextBefore + opts.ContextAfter + 1
	if gapThreshold < 1 {
		gapThreshold = 1
	}

	flush := func(path string) {
		st := states[path]
		if st == nil || st.cur == nil {
			return
		}
		if st.cur.LineNum == 0 {
			st.cur.LineNum = st.cur.ContextStartLine
		}
		st.matches = append(st.matches, *st.cur)
		st.cur = nil
	}

	for _, raw := range strings.Split(stdout.String(), "\n") {
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) < 3 {
			continue
		}
		path, lineStr, content := parts[0], parts[1], parts[2]
		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			continue
		}

		name := filepath.Base(path)
		if filenameRe != nil && !filenameRe.MatchString(name) {
			continue
		}
		if excludeMatches(name, opts.ExcludeGlobs, opts.CaseSensitive) {
			continue
		}

		st, ok := states[path]
		if !ok {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if opts.MinSizeBytes > 0 && info.Size() < opts.MinSizeBytes {
				continue
			}
			if opts.MaxSizeBytes > 0 && info.Size() > opts.MaxSizeBytes {
				continue
			}
			st = &rgFileState{entry: model.FileEntry{Path: path, Name: name, ModTime: info.ModTime(), Size: info.Size()}}
			states[path] = st
			order = append(order, path)
		}

		isMatchLine := contentRe.MatchString(content)

		switch {
		case st.cur == nil:
			st.cur = &model.ContentMatch{ContextStartLine: lineNum, ContextLines: []string{content}}
		case lineNum-st.lastLine > gapThreshold:
			flush(path)
			st.cur = &model.ContentMatch{ContextStartLine: lineNum, ContextLines: []string{content}}
		default:
			st.cur.ContextLines = append(st.cur.ContextLines, content)
		}
		if isMatchLine && st.cur.LineNum == 0 {
			st.cur.LineNum = lineNum
		}
		st.lastLine = lineNum
	}

	found := 0
	for _, path := range order {
		flush(path)
		st := states[path]
		if len(st.matches) == 0 {
			continue
		}
		found++
		select {
		case results <- model.FileResult{FileEntry: st.entry, Matches: st.matches}:
		case <-ctx.Done():
			return ctx.Err()
		}
		if progress != nil {
			select {
			case progress <- Progress{Searched: found, Found: found, Total: len(order)}:
			default:
			}
		}
	}
	return nil
}
