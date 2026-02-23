package ai

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer wraps the tiktoken library for counting tokens.
type Tokenizer struct {
	encoding *tiktoken.Tiktoken
}

// NewTokenizer creates a new Tokenizer for the specified model.
// Defaults to cl100k_base (GPT-4/3.5) if model is unknown.
func NewTokenizer(model string) (*Tokenizer, error) {
	// Llama 3 models often use cl100k_base or similar for estimation
	// MLX models usually have their own tokenizer, but cl100k_base is a good proxy
	// for OpenAI-compatible API responses if we don't have the exact vocab.
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fallback to cl100k_base if model mapping is missing
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, fmt.Errorf("failed to get encoding: %w", err)
		}
	}
	return &Tokenizer{encoding: enc}, nil
}

// CountTokens returns the number of tokens in the given text.
func (t *Tokenizer) CountTokens(text string) int {
	return len(t.encoding.Encode(text, nil, nil))
}
