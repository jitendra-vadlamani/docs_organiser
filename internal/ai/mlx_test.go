package ai

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"Simple name",
			"Document Title",
			"Document Title",
		},
		{
			"With slash",
			"Programming Languages / ANSI C",
			"Programming Languages _ ANSI C",
		},
		{
			"With backslash",
			"Folder\\File",
			"Folder_File",
		},
		{
			"With special characters",
			"What? * Is | This:",
			"What_ _ Is _ This",
		},
		{
			"Unicode slash",
			"Title\u2215WithSlash", // Division slash
			"Title_WithSlash",
		},
		{
			"Leading/trailing dots and spaces",
			"  .Hidden File.  ",
			"Hidden File",
		},
		{
			"Multiple underscores",
			"A///B___C",
			"A_B_C",
		},
		{
			"Empty string",
			"",
			"unnamed",
		},
		{
			"Only special characters",
			"/?*:",
			"unnamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
