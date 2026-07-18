package search

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"github.com/ledongthuc/pdf"
)

// extractPDFText extracts a PDF's text content, preferring the external
// `pdftotext` binary (from Poppler, better layout fidelity) when available,
// and falling back to a pure-Go extractor otherwise -- the same two-tier
// strategy the original app used with pdftotext/pypdf.
func extractPDFText(ctx context.Context, pdftotextPath, path string) (string, error) {
	if pdftotextPath != "" {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(cctx, pdftotextPath, path, "-")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			return out.String(), nil
		}
		// Any pdftotext failure (missing, crashed, unsupported PDF) falls
		// through to the pure-Go extractor below.
	}
	return extractPDFTextGo(path)
}

func extractPDFTextGo(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(b); err != nil {
		return "", err
	}
	return buf.String(), nil
}
