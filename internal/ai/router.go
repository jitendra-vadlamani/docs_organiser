package ai

import (
	"context"
	"fmt"
	"strings"
)

// TaskComplexity defines the level of reasoning required for a document.
type TaskComplexity string

const (
	ComplexitySimple  TaskComplexity = "simple"
	ComplexityComplex TaskComplexity = "complex"
)

// ModelRouter determines the best model path for a document.
type ModelRouter struct {
	engine *MLXEngine
}

func NewModelRouter(engine *MLXEngine) *ModelRouter {
	return &ModelRouter{engine: engine}
}

// ClassifyTask runs a fast pass to determine if a document is simple or complex.
func (r *ModelRouter) ClassifyTask(ctx context.Context, text string) (TaskComplexity, error) {
	// We use the default (fastest) model for classification
	modelName, _ := r.engine.selectBestModel(ctx)
	if modelName == "" {
		return ComplexitySimple, fmt.Errorf("no model available for classification")
	}

	// Truncate text for a quick classification pass (e.g., first 500 tokens)
	snippet := r.engine.ctxMgr.Truncate(text, 500, StrategySlidingWindow)

	prompt := fmt.Sprintf(`Analyze the document snippet and determine if it requires "simple" or "complex" reasoning for categorization.
Simple: Standard receipts, clear invoices, brief letters, simple markdown/txt files.
Complex: Technical papers, multi-page legal contracts, unstructured notes, or documents with ambiguous context.

Return ONLY the word "simple" or "complex".

Document Snippet:
%s`, snippet)

	req := chatRequest{
		Model:       modelName,
		Messages:    []message{{Role: "user", Content: prompt}},
		Stream:      false,
		Temperature: 0.0, // Strict deterministic output
	}

	// We'd use the engine's internal client here (simplified for draft)
	resp, err := r.engine.llm.CreateChatCompletion(ctx, req)
	if err != nil {
		return ComplexitySimple, err
	}

	if len(resp.Choices) == 0 {
		return ComplexitySimple, fmt.Errorf("empty classifier response")
	}

	result := strings.ToLower(strings.TrimSpace(resp.Choices[0].Message.Content))
	if strings.Contains(result, "complex") {
		return ComplexityComplex, nil
	}

	return ComplexitySimple, nil
}

// SelectBestModel returns the model name and URL based on complexity and availability.
func (r *ModelRouter) SelectBestModel(ctx context.Context, complexity TaskComplexity) (string, string) {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()

	// Heuristic: If complex, look for "powerful" models first
	if complexity == ComplexityComplex {
		for _, m := range r.engine.models {
			if strings.Contains(strings.ToLower(m.Name), "gpt-4") ||
				strings.Contains(strings.ToLower(m.Name), "claude") ||
				strings.Contains(strings.ToLower(m.Name), "8b") { // 8B is "complex" relative to 1B
				// Check if available (simplified)
				return m.Name, m.URL
			}
		}
	}

	// Fallback to default/simplest
	return r.engine.selectBestModel(ctx)
}
