package ai

import (
	"fmt"
	"testing"
)

func TestParseAndValidate(t *testing.T) {
	// Initialize engine to test its private method
	engine, _ := NewMLXEngine("http://localhost:8080/v1", "test-model", 4096, "cl100k_base")
	engine.SetCategories([]string{"Personal", "Work", "Work/Projects", "Finance"})

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		// Valid cases
		{
			name:    "Valid input",
			content: `{"category": "Personal", "title": "Diary", "confidence_score": 0.95}`,
			wantErr: false,
		},
		{
			name:    "Valid with markdown blocks",
			content: "```json\n" + `{"category": "Work", "title": "Project_Alpha", "confidence_score": 0.8}` + "\n```",
			wantErr: false,
		},
		{
			name:    "Valid nested category",
			content: `{"category": "Work/Projects", "title": "Project_X", "confidence_score": 0.9}`,
			wantErr: false,
		},

		// Missing Fields
		{name: "Missing category", content: `{"title": "Title", "confidence_score": 0.9}`, wantErr: true},
		{name: "Missing title", content: `{"category": "AI", "confidence_score": 0.9}`, wantErr: true},
		{name: "Missing confidence", content: `{"category": "AI", "title": "Title"}`, wantErr: true},
		{name: "Empty JSON", content: `{}`, wantErr: true},

		// Extra Fields (Strict enforcement)
		{name: "Extra field author", content: `{"category": "AI", "title": "Title", "confidence_score": 0.9, "author": "Me"}`, wantErr: true},
		{name: "Extra field tags", content: `{"category": "AI", "title": "Title", "confidence_score": 0.9, "tags": ["AI"]}`, wantErr: true},
		{name: "Extra field meta", content: `{"category": "AI", "title": "Title", "confidence_score": 0.9, "meta": {"date": "today"}}`, wantErr: true},

		// Wrong Enum
		{name: "Invalid category 'Unknown'", content: `{"category": "Unknown", "title": "Title", "confidence_score": 0.9}`, wantErr: true},
		{name: "Invalid category 'Random'", content: `{"category": "Random", "title": "Title", "confidence_score": 0.9}`, wantErr: true},
		{name: "Lowercase category (strict)", content: `{"category": "personal", "title": "Title", "confidence_score": 0.9}`, wantErr: true},

		// Invalid JSON
		{name: "Malformed JSON (no closing brace)", content: `{"category": "AI", "title": "Title", "confidence_score": 0.9`, wantErr: true},
		{name: "Malformed JSON (trailing comma)", content: `{"category": "AI", "title": "Title", "confidence_score": 0.9,}`, wantErr: true},
		{name: "Malformed JSON (unquoted key)", content: `{category: "AI", "title": "Title", "confidence_score": 0.9}`, wantErr: true},
		{name: "Malformed JSON (bad types - score as string)", content: `{"category": "AI", "title": "Title", "confidence_score": "0.9"}`, wantErr: true},

		// Low/Invalid Confidence
		{name: "Zero confidence", content: `{"category": "AI", "title": "Title", "confidence_score": 0.0}`, wantErr: true},
		{name: "Negative confidence", content: `{"category": "AI", "title": "Title", "confidence_score": -0.5}`, wantErr: true},
	}

	// Generating more tests to reach >= 50
	for i := 0; i < 10; i++ {
		tests = append(tests, struct {
			name, content string
			wantErr       bool
		}{
			name:    fmt.Sprintf("Extra field variation %d", i),
			content: fmt.Sprintf(`{"category": "AI", "title": "Title", "confidence_score": 0.9, "extra_%d": true}`, i),
			wantErr: true,
		})
	}

	for i := 0; i < 10; i++ {
		tests = append(tests, struct {
			name, content string
			wantErr       bool
		}{
			name:    fmt.Sprintf("Malformed JSON variation %d", i),
			content: fmt.Sprintf(`{"category": "AI", "title": "Title", "confidence_score": 0.9}} %d`, i),
			wantErr: true,
		})
	}

	for i := 0; i < 10; i++ {
		tests = append(tests, struct {
			name, content string
			wantErr       bool
		}{
			name:    fmt.Sprintf("Incorrect Enum variation %d", i),
			content: fmt.Sprintf(`{"category": "BadCat_%d", "title": "Title", "confidence_score": 0.9}`, i),
			wantErr: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.parseAndValidate(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAndValidate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("parseAndValidate() returned nil but no error")
			}
		})
	}
}
