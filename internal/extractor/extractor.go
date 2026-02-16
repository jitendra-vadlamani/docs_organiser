package extractor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractText extracts up to 'limit' characters of text from the file at 'path'.
func ExtractText(path string, limit int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".pdf":
		return extractPDFText(path, limit)
	default:
		// Fallback for .txt, .md, and others
		return extractPlainText(path, limit)
	}
}

func extractPDFText(path string, limit int) (text string, err error) {
	// Panic recovery for the pdf library which sometimes panics on malformed files
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pdf library panicked while processing %s: %v", path, r)
		}
	}()

	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var content strings.Builder
	totalPage := r.NumPage()

	// Limit to first 50 pages to avoid massive memory consumption on huge PDFs
	maxPages := 50
	if totalPage > maxPages {
		totalPage = maxPages
	}

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		s, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}

		content.WriteString(s)

		// Check limit inside loop to exit early
		if content.Len() >= limit {
			break
		}
	}

	text = content.String()
	if len(text) > limit {
		text = text[:limit]
	}
	return text, nil
}

func extractPlainText(path string, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	return string(buf[:n]), nil
}
