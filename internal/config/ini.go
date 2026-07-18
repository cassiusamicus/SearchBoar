package config

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// iniDoc is a minimal, order-preserving INI document model that mirrors the
// on-disk behavior of Python's configparser.ConfigParser as used by the
// original SearchBoar app (optionxform = str, i.e. keys are case-preserved).
//
// It intentionally does NOT use a general-purpose Go INI library: the
// existing ~/.config/searchboar/config.ini was written by configparser,
// which (a) splits each "key = value" line on the first '=' only, and
// (b) only treats a line as a comment when '#'/';' is the first
// non-whitespace character on that line — it never strips inline
// '#'/';' after a value. Favorites keys are full filesystem paths and
// values can contain arbitrary characters, so both behaviors matter and
// are replicated exactly here rather than relying on a library's default
// (and possibly different) comment/quote handling.
type iniDoc struct {
	sectionOrder []string
	sections     map[string]*iniSection
}

type iniSection struct {
	keyOrder []string
	values   map[string]string
}

func newIniDoc() *iniDoc {
	return &iniDoc{sections: map[string]*iniSection{}}
}

func (d *iniDoc) section(name string) *iniSection {
	s, ok := d.sections[name]
	if !ok {
		s = &iniSection{values: map[string]string{}}
		d.sections[name] = s
		d.sectionOrder = append(d.sectionOrder, name)
	}
	return s
}

func (d *iniDoc) get(section, key string) (string, bool) {
	s, ok := d.sections[section]
	if !ok {
		return "", false
	}
	v, ok := s.values[key]
	return v, ok
}

func (s *iniSection) set(key, value string) {
	if _, exists := s.values[key]; !exists {
		s.keyOrder = append(s.keyOrder, key)
	}
	s.values[key] = value
}

// keys returns a section's keys in file order, or nil if the section
// doesn't exist.
func (d *iniDoc) keys(section string) []string {
	s, ok := d.sections[section]
	if !ok {
		return nil
	}
	return s.keyOrder
}

// parseIni reads an INI document, replicating configparser's default
// parsing rules (see iniDoc doc comment).
func parseIni(r io.Reader) (*iniDoc, error) {
	doc := newIniDoc()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var cur *iniSection
	var lastKey string
	var contLines []string

	flushContinuation := func() {
		if cur != nil && lastKey != "" && len(contLines) > 0 {
			cur.values[lastKey] = cur.values[lastKey] + "\n" + strings.Join(contLines, "\n")
		}
		contLines = nil
	}

	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		if trimmed == "" {
			flushContinuation()
			lastKey = ""
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			flushContinuation()
			lastKey = ""
			continue
		}
		if (raw[0] == ' ' || raw[0] == '\t') && cur != nil && lastKey != "" {
			contLines = append(contLines, trimmed)
			continue
		}
		flushContinuation()

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			name := trimmed[1 : len(trimmed)-1]
			cur = doc.section(name)
			lastKey = ""
			continue
		}

		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 || cur == nil {
			// Malformed or orphaned line (no section yet); skip rather
			// than error, matching a tolerant reader for hand-edited files.
			lastKey = ""
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		cur.set(key, val)
		lastKey = key
	}
	flushContinuation()

	return doc, scanner.Err()
}

// write serializes the document in the same shape Python's
// ConfigParser.write() produces: "[Section]\n", then "key = value\n" per
// entry, then a blank line after every section (including the last).
func (d *iniDoc) write(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, secName := range d.sectionOrder {
		if _, err := fmt.Fprintf(bw, "[%s]\n", secName); err != nil {
			return err
		}
		sec := d.sections[secName]
		for _, k := range sec.keyOrder {
			if _, err := fmt.Fprintf(bw, "%s = %s\n", k, sec.values[k]); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(bw); err != nil {
			return err
		}
	}
	return bw.Flush()
}
