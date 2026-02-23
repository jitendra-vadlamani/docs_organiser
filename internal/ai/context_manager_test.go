package ai

import (
	"strings"
	"testing"
)

func TestContextManager_Truncate(t *testing.T) {
	tokenizer, _ := NewTokenizer("gpt-4")
	cm := NewContextManager(tokenizer, 100)

	longText := strings.Repeat("hello ", 200) // Much more than 100 tokens

	t.Run("SlidingWindow", func(t *testing.T) {
		limit := 20
		truncated := cm.Truncate(longText, limit, StrategySlidingWindow)

		tokens := tokenizer.encoding.Encode(truncated, nil, nil)
		// We expect the result to be around the limit, plus some overhead for the "[... truncated ...]" text
		if len(tokens) > limit+10 { // Allow some slack for the marker
			t.Errorf("SlidingWindow: expected roughly %d tokens, got %d", limit, len(tokens))
		}
		if !strings.Contains(truncated, "[... truncated ...]") {
			t.Errorf("SlidingWindow: expected truncation marker")
		}
	})

	t.Run("MiddleExtraction", func(t *testing.T) {
		limit := 50
		truncated := cm.Truncate(longText, limit, StrategyMiddleExtraction)

		tokens := tokenizer.encoding.Encode(truncated, nil, nil)
		if len(tokens) > limit+10 {
			t.Errorf("MiddleExtraction: expected roughly %d tokens, got %d", limit, len(tokens))
		}
		if !strings.Contains(truncated, "[... content extracted ...]") {
			t.Errorf("MiddleExtraction: expected truncation marker")
		}
	})
}

func TestContextManager_Budgets(t *testing.T) {
	tokenizer, _ := NewTokenizer("gpt-4")
	cm := NewContextManager(tokenizer, 1000)

	s, e, c, o := cm.GetBudgets()

	if s != 100 {
		t.Errorf("expected system budget 100, got %d", s)
	}
	if e != 200 {
		t.Errorf("expected examples budget 200, got %d", e)
	}
	if c != 600 {
		t.Errorf("expected content budget 600, got %d", c)
	}
	if o != 100 {
		t.Errorf("expected output budget 100, got %d", o)
	}
}
