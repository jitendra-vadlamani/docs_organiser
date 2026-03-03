package ai

import (
	"context"
	"docs_organiser/internal/config"
	"testing"
)

func TestCategorize_Robustness(t *testing.T) {
	tokenizer, _ := NewTokenizer("cl100k_base")
	ctxMgr := NewContextManager(tokenizer, 4096)

	t.Run("Success on first attempt", func(t *testing.T) {
		mock := &MockLLMClient{
			Responses: []*chatResponse{
				{
					Choices: []choice{{Message: message{Content: `{"category": "Work", "title": "Report", "confidence_score": 0.9}`}}},
					Usage:   Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				},
			},
		}

		engine := &MLXEngine{
			llm:             mock,
			models:          []config.ModelDefinition{{Name: "test-model", URL: "http://mock-api.com/v1"}},
			ctxMgr:          ctxMgr,
			validCategories: []string{"Work", "Personal"},
		}

		result, err := engine.Categorize(context.Background(), "some text")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.Analysis.Category != "Work" {
			t.Errorf("Expected category Work, got %s", result.Analysis.Category)
		}
		if result.Metadata.Attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", result.Metadata.Attempts)
		}
		if !result.Metadata.Success {
			t.Error("Expected metadata success to be true")
		}
	})

	t.Run("Retry on malformed JSON", func(t *testing.T) {
		mock := &MockLLMClient{
			Responses: []*chatResponse{
				{
					Choices: []choice{{Message: message{Content: `invalid json`}}},
					Usage:   Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
				},
				{
					Choices: []choice{{Message: message{Content: `{"category": "Personal", "title": "Photo", "confidence_score": 0.8}`}}},
					Usage:   Usage{PromptTokens: 15, CompletionTokens: 5, TotalTokens: 20},
				},
			},
		}

		engine := &MLXEngine{
			llm:             mock,
			models:          []config.ModelDefinition{{Name: "test-model", URL: "http://mock-api.com/v1"}},
			ctxMgr:          ctxMgr,
			validCategories: []string{"Work", "Personal"},
		}

		result, err := engine.Categorize(context.Background(), "some text")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result.Analysis.Category != "Personal" {
			t.Errorf("Expected category Personal, got %s", result.Analysis.Category)
		}
		if result.Metadata.Attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", result.Metadata.Attempts)
		}
	})
}
