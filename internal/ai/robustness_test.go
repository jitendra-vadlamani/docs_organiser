package ai

import (
	"context"
	"docs_organiser/internal/config"
	"fmt"
	"testing"
)

func TestCategorize_MalformedOutputRobustness(t *testing.T) {
	tokenizer, _ := NewTokenizer("cl100k_base")
	ctxMgr := NewContextManager(tokenizer, 4096)
	validCats := []string{"Work", "Personal", "Finance"}

	// Define malformed outputs to test robustness (Case table)
	testCases := []struct {
		name    string
		content string
	}{
		{"Missing Category", `{"title": "Report", "confidence_score": 0.9}`},
		{"Missing Title", `{"category": "Work", "confidence_score": 0.9}`},
		{"Missing Confidence", `{"category": "Work", "title": "Report"}`},
		{"Extra Fields", `{"category": "Work", "title": "Report", "confidence_score": 0.9, "reason": "found keyword"}`},
		{"Invalid Category", `{"category": "Vacation", "title": "Report", "confidence_score": 0.9}`},
		{"Zero Confidence", `{"category": "Work", "title": "Report", "confidence_score": 0}`},
		{"Negative Confidence", `{"category": "Work", "title": "Report", "confidence_score": -1.0}`},
		{"Markdown Block", "```json\n{\"category\": \"Work\", \"title\": \"Report\", \"confidence_score\": 0.9}\n```"},
		{"Trailing Data", `{"category": "Work", "title": "Report", "confidence_score": 0.9} something extra`},
		{"Double Braces", `{{{"category": "Work", "title": "Report", "confidence_score": 0.9}}}`},
		{"Array Instead of Object", `[{"category": "Work", "title": "Report", "confidence_score": 0.9}]`},
		{"Missing Comma", `{"category": "Work" "title": "Report", "confidence_score": 0.9}`},
		{"Unquoted Keys", `{category: "Work", title: "Report", confidence_score: 0.9}`},
		{"Single Quotes", `{'category': 'Work', 'title': 'Report', 'confidence_score': 0.9}`},
		{"Incomplete JSON", `{"category": "Work", "title": "Report"`},
		{"Empty String", ""},
		{"Null values", `{"category": null, "title": "Report", "confidence_score": 0.9}`},
		{"Boolean values", `{"category": true, "title": "Report", "confidence_score": 0.9}`},
		{"Nested objects", `{"category": {"name": "Work"}, "title": "Report", "confidence_score": 0.9}`},
		{"Unicode junk", `{"category": "Work", "title": "Report\u1234", "confidence_score": 0.9} 🚀`},
		{"Path injection cat", `{"category": "../../../etc/passwd", "title": "Report", "confidence_score": 0.9}`},
		{"Large confidence", `{"category": "Work", "title": "Report", "confidence_score": 99999}`},
		{"Title with slashes", `{"category": "Work", "title": "Sub/Folder/Report", "confidence_score": 0.9}`},
		{"Very long title", `{"category": "Work", "title": "VeryLongTitle_` + fmt.Sprintf("%0100d", 0) + `", "confidence_score": 0.9}`},
		{"HTML content", `<html><body>{"category": "Work", "title": "Report", "confidence_score": 0.9}</body></html>`},
		{"JS Block", `const data = {"category": "Work", "title": "Report", "confidence_score": 0.9};`},
		{"CSV style", `category,title,confidence\nWork,Report,0.9`},
		{"YAML style", `category: Work\ntitle: Report\nconfidence_score: 0.9`},
		{"Mixed content", `Here is the JSON: {"category": "Work", "title": "Report", "confidence_score": 0.9} hope it helps!`},
		{"Escaped quotes", `{"category": "Work", "title": "Report \"Quote\"", "confidence_score": 0.9}`},
		{"Control characters", `{"category": "Work\n", "title": "Report\r", "confidence_score": 0.9}`},
		{"Malformed confidence string", `{"category": "Work", "title": "Report", "confidence_score": "high"}`},
		{"Array of categories", `{"category": ["Work", "Finance"], "title": "Report", "confidence_score": 0.9}`},
		{"Emoji category", `{"category": "💼", "title": "Report", "confidence_score": 0.9}`},
		{"Whitespace overkill", ` {   "category"  :  "Work"  ,  "title"  :  "Report"  ,  "confidence_score"  :  0.9  } `},
		{"No curly braces", `"category": "Work", "title": "Report", "confidence_score": 0.9`},
		{"Multiple JSON objects", `{"category": "Work", "title": "R1", "confidence_score": 0.9} {"category": "Personal", "title": "R2", "confidence_score": 0.1}`},
		{"Only confidence", `{"confidence_score": 0.9}`},
		{"Capitalized keys", `{"Category": "Work", "Title": "Report", "Confidence_Score": 0.9}`},
		{"Snake case failure", `{"category_name": "Work", "file_title": "Report", "score": 0.9}`},
		{"Non-ASCII Category", `{"category": "Trabajo", "title": "Report", "confidence_score": 0.9}`},
		{"Leading zero confidence", `{"category": "Work", "title": "Report", "confidence_score": .9}`},
		{"Title as number", `{"category": "Work", "title": 12345, "confidence_score": 0.9}`},
		{"Confidence as string", `{"category": "Work", "title": "Report", "confidence_score": "0.9"}`},
		{"Unicode separators", `{"category":"Work"\u1680"title":"Report"}`},
		{"Very large JSON body", `{"junk": "` + fmt.Sprintf("%0500d", 0) + `", "category": "Work", "title": "R1", "confidence_score": 0.9}`},
		{"Invisible characters", `{"category": "Work\u200B", "title": "Report", "confidence_score": 0.9}`},
		{"Zero confidence string", `{"category": "Work", "title": "Report", "confidence_score": 0.000}`},
		{"Scientific notation", `{"category": "Work", "title": "Report", "confidence_score": 9e-1}`},
		{"Infinite loop risk content", `{"category": "Work", "title": "{{{", "confidence_score": 0.9}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock LLM provides the malformed content initially, then potentially correct content on retry
			// But for "robustness", we want to ensure it handles the malformed part without CRASHING.
			// The orchestration logic will retry. We want to verify that even if it fails all retries,
			// it returns a graceful fallback (Misc) and not a crash/error that bubbles up.

			mock := &MockLLMClient{
				Responses: []*chatResponse{
					{Choices: []choice{{Message: message{Role: "assistant", Content: tc.content}}}},
					{Choices: []choice{{Message: message{Role: "assistant", Content: tc.content}}}},
					{Choices: []choice{{Message: message{Role: "assistant", Content: tc.content}}}},
				},
			}

			engine := &MLXEngine{
				llm:             mock,
				models:          []config.ModelDefinition{{Name: "test-model", URL: "http://mock-api.com/v1/chat/completions"}},
				ctxMgr:          ctxMgr,
				validCategories: validCats,
			}

			result, err := engine.Categorize(context.Background(), "test text")

			// If it fails after 3 attempts (initial + 2 retries), result.Analysis should be fallback
			if result == nil {
				t.Fatalf("Result should never be nil")
			}

			// It should never panic.
			// It might return an error after retries fail, which is fine as long as we have a fallback.
			if err != nil {
				// Verify fallback values
				if result.Analysis.Category != "Misc" {
					t.Errorf("Expected fallback category Misc for test %s, got %s", tc.name, result.Analysis.Category)
				}
			}
		})
	}
}
