package ai

import (
	"context"
	"fmt"
)

// MockLLMClient allows for deterministic testing of the orchestration layer.
type MockLLMClient struct {
	Responses []*chatResponse
	Errors    []error
	CallCount int
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, req chatRequest) (*chatResponse, error) {
	if m.CallCount >= len(m.Responses) && m.CallCount >= len(m.Errors) {
		return nil, fmt.Errorf("no more mock responses configured")
	}

	var resp *chatResponse
	var err error

	if m.CallCount < len(m.Responses) {
		resp = m.Responses[m.CallCount]
	}

	if m.CallCount < len(m.Errors) {
		err = m.Errors[m.CallCount]
	}

	m.CallCount++
	return resp, err
}
func (m *MockLLMClient) Endpoint() string {
	return "http://mock-api.com/v1/chat/completions"
}
