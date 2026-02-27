package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MLXEngine handles interaction with the MLX model server.
type MLXEngine struct {
	apiURL          string
	modelName       string
	client          *http.Client
	ctxMgr          *ContextManager
	validCategories []string
}

// AnalysisResult is the structure we expect from the LLM.
type AnalysisResult struct {
	Category        string  `json:"category"`
	Title           string  `json:"title"`
	ConfidenceScore float64 `json:"confidence_score"`
}

// DefaultCategories defines the fallback destination folders.
var DefaultCategories = []string{
	"Personal", "Work", "Finance", "Health", "Education", "Technical",
	"Travel", "Legal", "Projects", "Receipts", "Misc",
}

// OpenAI-compatible request structure
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI-compatible response structure
type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

// NewMLXEngine creates a new instance of the engine.
// apiURL should be the base URL, e.g., "http://localhost:8080/v1"
// modelName is the model identifier to send in the request
// maxTokens is the context window size
// encodingName is the tiktoken encoding to use for token counting
func NewMLXEngine(apiURL, modelName string, maxTokens int, encodingName string) (*MLXEngine, error) {
	if apiURL == "" {
		apiURL = "http://localhost:8080/v1"
	}
	// Ensure URL ends with /chat/completions if not provided
	if !strings.HasSuffix(apiURL, "/chat/completions") {
		apiURL = strings.TrimRight(apiURL, "/") + "/chat/completions"
	}

	tokenizer, err := NewTokenizer(encodingName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tokenizer: %w", err)
	}

	// Use provided max tokens for context management
	ctxMgr := NewContextManager(tokenizer, maxTokens)

	return &MLXEngine{
		apiURL:    apiURL,
		modelName: modelName,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		ctxMgr:          ctxMgr,
		validCategories: DefaultCategories,
	}, nil
}

// SetCategories overrides the allowed categorization folders.
func (e *MLXEngine) SetCategories(categories []string) {
	if len(categories) > 0 {
		e.validCategories = categories
	}
}

// Categorize analyzes the text and returns a folder category and cleaned filename.
func (e *MLXEngine) Categorize(ctx context.Context, text string) (*AnalysisResult, error) {
	systemPrompt := fmt.Sprintf(`You are an intelligent file organization assistant. Analyze the document text and return a SINGLE JSON object.
Required format: {"category": "Specific_Category_Name", "title": "Clean_Filename_No_Ext", "confidence_score": 0.0-1.0}
Strictly choose category from: %s
Nested paths like "Parent/Child" are valid if they exist in the list above.
Required confidence_score: a float between 0.0 and 1.0.
Do NOT return extra fields. Do NOT return markdown. Do NOT return extra text.`, strings.Join(e.validCategories, ", "))

	systemBudget, _, contentBudget, _ := e.ctxMgr.GetBudgets()
	systemPrompt = e.ctxMgr.Truncate(systemPrompt, systemBudget, StrategySlidingWindow)

	currentTokens := e.ctxMgr.tokenizer.CountTokens(text)
	if currentTokens > contentBudget {
		summary, err := e.MapReduceSummarize(ctx, text, contentBudget)
		if err != nil {
			text = e.ctxMgr.Truncate(text, contentBudget, StrategyMiddleExtraction)
		} else {
			text = summary
		}
	}

	userPrompt := fmt.Sprintf("Document text snippet:\n%s", text)

	var lastErr error
	maxRetries := 2

	// Correction loop
	for attempt := 0; attempt <= maxRetries; attempt++ {
		messages := []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		// If this is a retry, inject the previous error
		if attempt > 0 && lastErr != nil {
			messages = append(messages, message{
				Role:    "assistant",
				Content: "Previous attempt failed validation.",
			})
			messages = append(messages, message{
				Role:    "user",
				Content: fmt.Sprintf("Your previous response was invalid: %v. Please provide a strictly valid JSON object following the schema.", lastErr),
			})
		}

		reqBody := chatRequest{
			Model:       e.modelName,
			Messages:    messages,
			Stream:      false,
			Temperature: 0.1,
		}

		result, err := e.executeCategorization(ctx, reqBody)
		if err == nil {
			return result, nil
		}

		lastErr = err
		// log.Printf("[DEBUG] Attempt %d failed: %v", attempt+1, err)
	}

	// Escalation fallback path
	return &AnalysisResult{
		Category:        "Misc",
		Title:           "Unknown_Doc",
		ConfidenceScore: 0.0,
	}, fmt.Errorf("failed to get valid structured output after %d retries: %w", maxRetries, lastErr)
}

func (e *MLXEngine) executeCategorization(ctx context.Context, reqBody chatRequest) (*AnalysisResult, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to MLX server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MLX server error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	return e.parseAndValidate(content)
}

func (e *MLXEngine) parseAndValidate(content string) (*AnalysisResult, error) {
	content = cleanJSON(content)

	// Use decoder with DisallowUnknownFields for strict validation
	var result AnalysisResult
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid JSON or unexpected fields: %w", err)
	}

	// Check for trailing data to ensure strict validation
	var dummy json.RawMessage
	if err := dec.Decode(&dummy); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON object")
	}

	// Required fields validation
	if result.Category == "" {
		return nil, fmt.Errorf("missing required field: category")
	}
	if result.Title == "" {
		return nil, fmt.Errorf("missing required field: title")
	}
	if result.ConfidenceScore <= 0 {
		// Even if provided, if it's 0 it might be missing or explicitly low
		// We'll treat <= 0 as invalid per requirements "Required confidence_score"
		return nil, fmt.Errorf("missing or invalid confidence_score: %v", result.ConfidenceScore)
	}

	// Enum validation
	valid := false
	for _, c := range e.validCategories {
		if result.Category == c {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid category: %s (must be one of %v)", result.Category, e.validCategories)
	}

	result.Category = SanitizeCategory(result.Category)
	result.Title = SanitizeFilename(result.Title)

	return &result, nil
}

// SanitizeCategory removes dangerous characters but allows forward slashes for nested paths.
func SanitizeCategory(s string) string {
	// Allow / but sanitize other path characters
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")

	var builder strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == ' ' || r == '/' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}

	result := strings.TrimSpace(builder.String())
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	// Avoid double slashes
	for strings.Contains(result, "//") {
		result = strings.ReplaceAll(result, "//", "/")
	}
	result = strings.Trim(result, ". _/")

	if result == "" {
		result = "unnamed"
	}
	return result
}

// SanitizeFilename removes dangerous characters from AI-generated strings
func SanitizeFilename(s string) string {
	// 1. Initial cleanup: replace common path separators and problematic characters
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")

	// 2. Filter characters: allow alphanumeric, space, underscore, hyphen, and dot.
	// Everything else becomes an underscore.
	var builder strings.Builder
	for _, r := range s {
		// Basic Alphanumeric + a few safe symbols
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == ' ' {
			builder.WriteRune(r)
		} else {
			// Catch-all for any other character (unicode slashes, control chars, etc.)
			builder.WriteRune('_')
		}
	}

	result := strings.TrimSpace(builder.String())
	// Collapse multiple underscores and spaces
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	// Avoid leading/trailing dots/spaces/underscores which can be problematic
	result = strings.Trim(result, ". _")

	if result == "" {
		result = "unnamed"
	}
	return result
}

// MapReduceSummarize reduces a large text into a shorter summary that fits within limit tokens.
func (e *MLXEngine) MapReduceSummarize(ctx context.Context, text string, limit int) (string, error) {
	currentTokens := e.ctxMgr.tokenizer.CountTokens(text)
	if currentTokens <= limit {
		return text, nil
	}

	// 1. Chunking: Split text into chunks that fit in the model's window
	// We use 80% of content budget for chunks to leave room for the summarization prompt
	_, _, contentBudget, _ := e.ctxMgr.GetBudgets()
	chunkSize := int(float64(contentBudget) * 0.8)
	chunks := e.ctxMgr.Chunk(text, chunkSize)

	// 2. Map: Summarize each chunk
	var summaries []string
	for i, chunk := range chunks {
		summary, err := e.summarizeChunk(ctx, chunk, i+1, len(chunks))
		if err != nil {
			return "", fmt.Errorf("failed to summarize chunk %d/%d: %w", i+1, len(chunks), err)
		}
		summaries = append(summaries, summary)
	}

	// 3. Reduce: Combine and recursively summarize if needed
	combined := strings.Join(summaries, "\n\n")
	combinedTokens := e.ctxMgr.tokenizer.CountTokens(combined)

	if combinedTokens > limit {
		// Recursive reduction
		return e.MapReduceSummarize(ctx, combined, limit)
	}

	return combined, nil
}

func (e *MLXEngine) summarizeChunk(ctx context.Context, text string, index, total int) (string, error) {
	prompt := fmt.Sprintf("Summarize the following document part (%d/%d). Keep key technical details, names, and core topics relevant for categorization:\n\n%s", index, total, text)

	reqBody := chatRequest{
		Model: e.modelName,
		Messages: []message{
			{Role: "system", Content: "You are a concise summarization assistant. Provide a brief but information-dense summary."},
			{Role: "user", Content: prompt},
		},
		Stream:      false,
		Temperature: 0.1,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// cleanJSON attempts to extract the valid JSON object
func cleanJSON(s string) string {
	// 1. Remove markdown code blocks
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	// 2. Remove common LLM suffixes
	s = strings.ReplaceAll(s, "<|eot_id|>", "")

	// 3. Find first '{' and last '}'
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")

	if start != -1 && end != -1 && end > start {
		s = s[start : end+1]
	}

	return strings.TrimSpace(s)
}
