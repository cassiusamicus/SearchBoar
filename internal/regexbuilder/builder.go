// Package regexbuilder holds the UI-independent logic behind SearchBoar's
// "Expr. Wizard" dialog: turning a pattern type + plain text into a regex,
// and live-testing that regex against sample text.
package regexbuilder

import (
	"fmt"
	"regexp"
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
	CustomRegex: "Custom regex",
}

// PatternTypes lists every pattern type in the order the original dialog's
// combo box presented them.
var PatternTypes = []PatternType{ExactMatch, Contains, StartsWith, EndsWith, Extension, Email, IP, CustomRegex}

func (t PatternType) String() string { return patternTypeLabels[t] }

// NeedsText reports whether the pattern type takes free-text input (Email
// and IP use a fixed pattern regardless of Text, matching the original
// dialog, which grayed out the text entry for those two options).
func (t PatternType) NeedsText() bool {
	return t != Email && t != IP
}

const (
	emailPattern = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	ipPattern    = `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`
)

// Builder holds the current state of the regex wizard dialog.
type Builder struct {
	Type          PatternType
	Text          string
	CaseSensitive bool
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
