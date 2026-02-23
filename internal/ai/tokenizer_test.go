package ai

import (
	"testing"
)

func TestNewTokenizer(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{
			"Valid OpenAI model",
			"gpt-4",
			false,
		},
		{
			"Unknown model fallback",
			"unknown-model-123",
			false, // Should fallback to cl100k_base
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTokenizer(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTokenizer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTokenizer_CountTokens(t *testing.T) {
	tokenizer, err := NewTokenizer("gpt-4")
	if err != nil {
		t.Fatalf("failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name string
		text string
		want int
	}{
		{
			"Empty string",
			"",
			0,
		},
		{
			"Simple sentence",
			"Hello, world!",
			4, // "Hello", ",", " world", "!" (cl100k_base)
		},
		{
			"Repeated text",
			"test test test",
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizer.CountTokens(tt.text)
			if got != tt.want {
				t.Errorf("CountTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}
