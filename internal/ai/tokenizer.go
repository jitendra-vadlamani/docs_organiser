package ai

import (
	"fmt"

	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer wraps the tiktoken library for counting tokens.
type Tokenizer struct {
	encoding *tiktoken.Tiktoken
}

// NewTokenizer creates a new Tokenizer using the specified encoding (e.g., cl100k_base).
func NewTokenizer(encodingName string) (*Tokenizer, error) {
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, fmt.Errorf("failed to get encoding '%s': %w", encodingName, err)
	}
	return &Tokenizer{encoding: enc}, nil
}

// CountTokens returns the number of tokens in the given text.
func (t *Tokenizer) CountTokens(text string) int {
	return len(t.encoding.Encode(text, nil, nil))
}
