package search

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
)

// extractDOCXText extracts plain text from a .docx file's word/document.xml
// by walking the XML token stream and collecting character data, using only
// the standard library. The original Python app exposed a DOCX filetype
// checkbox but never actually implemented DOCX content extraction (files
// were misdetected as binary and silently skipped); this is a real fix, not
// a preserved behavior.
func extractDOCXText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return decodeDocumentXML(rc)
	}
	return "", errors.New("word/document.xml not found in docx")
}

func decodeDocumentXML(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var buf bytes.Buffer
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if buf.Len() > 0 {
					buf.WriteByte('\n')
				}
			case "tab":
				buf.WriteByte('\t')
			}
		}
	}
	return buf.String(), nil
}
