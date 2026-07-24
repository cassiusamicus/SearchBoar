// Package regexbuilder holds the UI-independent logic behind SearchBoar's
// "Expr. Wizard" dialog: turning a pattern type + plain text into a regex,
// and live-testing that regex against sample text.
package regexbuilder

import (
	"fmt"
	"regexp"
	"strings"
)

type PatternType int

const (
	ExactMatch PatternType = iota
	Contains
	StartsWith
	EndsWith
	Extension
	Email
	IP
	AllWords
	AnyWord
	NearWords
	CustomRegex
)

var patternTypeLabels = map[PatternType]string{
	ExactMatch:  "Exact match",
	Contains:    "Contains text",
	StartsWith:  "Starts with",
	EndsWith:    "Ends with",
	Extension:   "File extension",
	Email:       "Email address",
	IP:          "IP address",
	AllWords:    "All of these words (AND)",
	AnyWord:     "Any of these words (OR)",
	NearWords:   "These words near each other",
	CustomRegex: "Custom regex",
}

// PatternTypes lists every pattern type in the order the original dialog's
// combo box presented them, with AllWords/AnyWord/NearWords added after the
// original set and CustomRegex kept last as the escape hatch for anyone who
// wants to write a regex directly.
var PatternTypes = []PatternType{ExactMatch, Contains, StartsWith, EndsWith, Extension, Email, IP, AllWords, AnyWord, NearWords, CustomRegex}

func (t PatternType) String() string { return patternTypeLabels[t] }

// NeedsText reports whether the pattern type takes free-text input (Email
// and IP use a fixed pattern regardless of Text, matching the original
// dialog, which grayed out the text entry for those two options).
func (t PatternType) NeedsText() bool {
	return t != Email && t != IP
}

// IsWordList reports whether Text holds a comma-separated list of words
// (AllWords/AnyWord) rather than a single string.
func (t PatternType) IsWordList() bool {
	return t == AllWords || t == AnyWord
}

// IsProximity reports whether the pattern type needs the dialog's second
// word and distance fields in addition to Text.
func (t PatternType) IsProximity() bool {
	return t == NearWords
}

const (
	emailPattern = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	ipPattern    = `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`

	// defaultProximityDistance is how many intervening words NearWords
	// allows when Distance is left unset (<= 0).
	defaultProximityDistance = 10
)

// Builder holds the current state of the regex wizard dialog.
type Builder struct {
	Type          PatternType
	Text          string
	CaseSensitive bool

	// Text2 and Distance are only used by NearWords: Text and Text2 are the
	// two words, Distance is the maximum number of other words allowed
	// between them (either order counts as a match).
	Text2    string
	Distance int
}

// splitWordList splits a comma-separated word list the way AllWords/AnyWord
// take their input, trimming whitespace and dropping empty entries -- the
// same forgiving parsing as the Type Wizard's "Other extensions" field, so
// "cat, dog,,  bird" still comes out as three clean words.
func splitWordList(s string) []string {
	var words []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			words = append(words, part)
		}
	}
	return words
}

// permutations returns every ordering of words. Used by AllWords: Go's
// regexp package (RE2) supports neither lookahead nor backreferences, so
// "every word appears, in any order" has to be spelled out as an
// alternation over every ordering rather than expressed directly. This is
// only fed a handful of words at a time (an AND list is realistically 2-4
// terms), so the factorial blowup never gets large enough to matter.
func permutations(words []string) [][]string {
	if len(words) <= 1 {
		return [][]string{words}
	}
	var result [][]string
	for i, w := range words {
		rest := make([]string, 0, len(words)-1)
		rest = append(rest, words[:i]...)
		rest = append(rest, words[i+1:]...)
		for _, perm := range permutations(rest) {
			result = append(result, append([]string{w}, perm...))
		}
	}
	return result
}

// Generate produces the regex for the current state.
//
// Note: the original app's "Exact match" and "Contains text" options
// generated the identical unanchored regex (a pre-existing bug), making
// "Exact match" not actually exact. This rewrite fixes that: ExactMatch is
// anchored with ^...$, Contains remains unanchored.
func (b Builder) Generate() (string, error) {
	var pattern string
	switch b.Type {
	case ExactMatch:
		pattern = "^" + regexp.QuoteMeta(b.Text) + "$"
	case Contains:
		pattern = regexp.QuoteMeta(b.Text)
	case StartsWith:
		pattern = "^" + regexp.QuoteMeta(b.Text)
	case EndsWith:
		pattern = regexp.QuoteMeta(b.Text) + "$"
	case Extension:
		pattern = `\.` + regexp.QuoteMeta(b.Text) + "$"
	case Email:
		pattern = emailPattern
	case IP:
		pattern = ipPattern
	case AllWords:
		words := splitWordList(b.Text)
		if len(words) == 0 {
			return "", fmt.Errorf("enter at least one word")
		}
		var alts []string
		for _, perm := range permutations(words) {
			quoted := make([]string, len(perm))
			for i, w := range perm {
				quoted[i] = regexp.QuoteMeta(w)
			}
			alts = append(alts, strings.Join(quoted, ".*"))
		}
		pattern = strings.Join(alts, "|")
	case AnyWord:
		words := splitWordList(b.Text)
		if len(words) == 0 {
			return "", fmt.Errorf("enter at least one word")
		}
		quoted := make([]string, len(words))
		for i, w := range words {
			quoted[i] = regexp.QuoteMeta(w)
		}
		pattern = strings.Join(quoted, "|")
	case NearWords:
		w1, w2 := strings.TrimSpace(b.Text), strings.TrimSpace(b.Text2)
		if w1 == "" || w2 == "" {
			return "", fmt.Errorf("enter both words")
		}
		dist := b.Distance
		if dist <= 0 {
			dist = defaultProximityDistance
		}
		q1, q2 := regexp.QuoteMeta(w1), regexp.QuoteMeta(w2)
		between := fmt.Sprintf(`(?:\W+\w+){0,%d}`, dist)
		pattern = fmt.Sprintf(`\b%s\b%s\W+\b%s\b|\b%s\b%s\W+\b%s\b`, q1, between, q2, q2, between, q1)
	case CustomRegex:
		pattern = b.Text
	default:
		return "", fmt.Errorf("unknown pattern type %d", b.Type)
	}

	// Case-insensitivity wraps everything except CustomRegex, which assumes
	// the user manages case-sensitivity themselves within their own pattern.
	if !b.CaseSensitive && b.Type != CustomRegex {
		pattern = "(?i)" + pattern
	}
	return pattern, nil
}

// Test compiles the current pattern and reports how many times it matches
// sample, mirroring the wizard's live "Test Pattern" panel.
func (b Builder) Test(sample string) (matchCount int, err error) {
	pattern, err := b.Generate()
	if err != nil {
		return 0, err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, err
	}
	return len(re.FindAllStringIndex(sample, -1)), nil
}
