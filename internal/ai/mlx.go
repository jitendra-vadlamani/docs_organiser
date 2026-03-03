package ai

import (
	"bytes"
	"context"
	"docs_organiser/internal/config"
	"docs_organiser/internal/observability"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LLMClient defines the interface for interacting with any LLM server.
type LLMClient interface {
	CreateChatCompletion(ctx context.Context, req chatRequest) (*chatResponse, error)
	Endpoint() string
}

// NetLLMClient is the standard HTTP implementation of LLMClient.
type NetLLMClient struct {
	client *http.Client
	apiURL string
}

func (c *NetLLMClient) CreateChatCompletion(ctx context.Context, req chatRequest) (*chatResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

func (c *NetLLMClient) Endpoint() string {
	return c.apiURL
}

// MLXEngine handles interaction with the model servers.
type MLXEngine struct {
	llm              LLMClient
	models           []config.ModelDefinition
	defaultModelName string
	ctxMgr           *ContextManager
	validCategories  []string
	mu               sync.RWMutex
}

// CategorizationMetadata holds telemetry and usage data for a request.
type CategorizationMetadata struct {
	Model          string        `json:"model"`
	Latency        time.Duration `json:"latency"`
	PromptTokens   int           `json:"prompt_tokens"`
	ResponseTokens int           `json:"response_tokens"`
	TotalTokens    int           `json:"total_tokens"`
	TruncationType string        `json:"truncation_type"`
	Attempts       int           `json:"attempts"`
	Success        bool          `json:"success"`
}

// CategorizationResult combines the AI response with metadata.
type CategorizationResult struct {
	Analysis *AnalysisResult         `json:"analysis"`
	Metadata *CategorizationMetadata `json:"metadata"`
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
	Usage   Usage    `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type choice struct {
	Message message `json:"message"`
}

// NewMLXEngine creates a new instance of the engine.
func NewMLXEngine(apiURL string, allowedModels []config.ModelDefinition, maxTokens int, encodingName string) (*MLXEngine, error) {
	tokenizer, err := NewTokenizer(encodingName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tokenizer: %w", err)
	}

	// Use provided max tokens for context management
	ctxMgr := NewContextManager(tokenizer, maxTokens)

	if len(allowedModels) == 0 {
		allowedModels = []config.ModelDefinition{
			{Name: "mlx-community/Llama-3.2-1B-Instruct-4bit", URL: apiURL},
		}
	}

	return &MLXEngine{
		llm: &NetLLMClient{
			client: &http.Client{
				Timeout: 60 * time.Second,
			},
		},
		models:          allowedModels,
		ctxMgr:          ctxMgr,
		validCategories: DefaultCategories,
	}, nil
}

// SetCategories overrides the allowed categorization folders.
func (e *MLXEngine) SetCategories(categories []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(categories) > 0 {
		e.validCategories = categories
	}
}

// GetCategories returns the current list of allowed categories.
func (e *MLXEngine) GetCategories() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.validCategories
}

// ContextWindow returns the maximum tokens allowed for the current context.
func (e *MLXEngine) ContextWindow() int {
	return e.ctxMgr.maxTokens
}

// SetDefaultModel sets the preferred model to use if available.
func (e *MLXEngine) SetDefaultModel(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.defaultModelName = name
}

// SetAllowedModels updates the pool of models used for requests.
func (e *MLXEngine) SetAllowedModels(models []config.ModelDefinition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(models) > 0 {
		e.models = models
	}
}

// GetCurrentURL returns the URL of a specific model by name.
func (e *MLXEngine) GetURLForModel(name string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, m := range e.models {
		if m.Name == name {
			return m.URL
		}
	}
	return ""
}

// GetAvailableModelsForURL probes a specific endpoint for active models.
func (e *MLXEngine) GetAvailableModelsForURL(ctx context.Context, apiURL string) ([]string, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("empty API URL")
	}
	baseURL := strings.TrimRight(apiURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models from %s: status %d", apiURL, resp.StatusCode)
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range data.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

func (e *MLXEngine) selectBestModel(ctx context.Context) (string, string) {
	e.mu.RLock()
	models := e.models
	defaultModel := e.defaultModelName
	e.mu.RUnlock()

	// 1. Try default model first
	if defaultModel != "" {
		for _, m := range models {
			if m.Name == defaultModel {
				available, err := e.GetAvailableModelsForURL(ctx, m.URL)
				if err == nil {
					for _, avail := range available {
						if m.Name == avail {
							return m.Name, m.URL
						}
					}
				}
				break
			}
		}
	}

	// 2. Otherwise try any active model in the pool
	for _, m := range models {
		available, err := e.GetAvailableModelsForURL(ctx, m.URL)
		if err != nil {
			continue // Try next provider
		}
		for _, avail := range available {
			if m.Name == avail {
				return m.Name, m.URL
			}
		}
	}

	// Fallback to first configured if nothing is active
	if len(models) > 0 {
		return models[0].Name, models[0].URL
	}
	return "", ""
}

// Categorize analyzes the text and returns a folder category and cleaned filename.
func (e *MLXEngine) Categorize(ctx context.Context, text string) (*CategorizationResult, error) {
	startTime := time.Now()
	modelName, apiURL := e.selectBestModel(ctx)
	if modelName == "" {
		return nil, fmt.Errorf("no model available")
	}

	// For NetLLMClient, we need to inject the specific URL for this request
	if net, ok := e.llm.(*NetLLMClient); ok {
		fullURL := strings.TrimRight(apiURL, "/")
		if !strings.HasSuffix(fullURL, "/chat/completions") {
			fullURL += "/chat/completions"
		}
		net.apiURL = fullURL
	}

	metadata := &CategorizationMetadata{
		Model:          modelName,
		TruncationType: "none",
		Success:        false,
	}

	systemPrompt := fmt.Sprintf(`You are an intelligent file organization assistant. Analyze the document text and return a SINGLE JSON object.
Required format: {"category": "Specific_Category_Name", "title": "Clean_Filename_No_Ext", "confidence_score": 0.0-1.0}
Strictly choose category from: %s
Nested paths like "Parent/Child" are valid if they exist in the list above.
Required confidence_score: a float between 0.0 and 1.0.
Do NOT return extra fields. Do NOT return markdown. Do NOT return extra text.`, strings.Join(e.validCategories, ", "))

	systemBudget, _, contentBudget, _ := e.ctxMgr.GetBudgets()
	systemPrompt = e.ctxMgr.Truncate(systemPrompt, systemBudget, StrategySlidingWindow)
	// We don't record sliding window for system prompt as it's static/small usually

	currentTokens := e.ctxMgr.tokenizer.CountTokens(text)
	if currentTokens > contentBudget {
		metadata.TruncationType = string(StrategyMapReduce)
		summary, err := e.MapReduceSummarize(ctx, text, contentBudget, modelName, apiURL)
		if err != nil {
			metadata.TruncationType = string(StrategyMiddleExtraction)
			text = e.ctxMgr.Truncate(text, contentBudget, StrategyMiddleExtraction)
		} else {
			text = summary
		}
		observability.TruncationEventsTotal.WithLabelValues(modelName, metadata.TruncationType).Inc()
	}

	userPrompt := fmt.Sprintf("Document text snippet:\n%s", text)

	var lastErr error
	maxRetries := 2

	// Correction loop
	for attempt := 0; attempt <= maxRetries; attempt++ {
		metadata.Attempts++
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
			Model:       modelName,
			Messages:    messages,
			Stream:      false,
			Temperature: 0.1,
		}

		chatResp, err := e.llm.CreateChatCompletion(ctx, reqBody)
		if err == nil && len(chatResp.Choices) > 0 {
			metadata.PromptTokens += chatResp.Usage.PromptTokens
			metadata.ResponseTokens += chatResp.Usage.CompletionTokens
			metadata.TotalTokens += chatResp.Usage.TotalTokens

			// Update Prometheus metrics
			observability.LLMTokensTotal.WithLabelValues(modelName, "prompt").Add(float64(chatResp.Usage.PromptTokens))
			observability.LLMTokensTotal.WithLabelValues(modelName, "completion").Add(float64(chatResp.Usage.CompletionTokens))
			observability.LLMTokensTotal.WithLabelValues(modelName, "total").Add(float64(chatResp.Usage.TotalTokens))

			content := chatResp.Choices[0].Message.Content
			result, parseErr := e.parseAndValidate(content)
			if parseErr == nil {
				metadata.Success = true
				metadata.Latency = time.Since(startTime)
				observability.LLMRequestDuration.WithLabelValues(modelName, "categorization").Observe(metadata.Latency.Seconds())
				return &CategorizationResult{
					Analysis: result,
					Metadata: metadata,
				}, nil
			}
			lastErr = parseErr
		} else {
			lastErr = err
		}
	}

	metadata.Latency = time.Since(startTime)
	// Escalation fallback path
	return &CategorizationResult{
		Analysis: &AnalysisResult{
			Category:        "Misc",
			Title:           "Unknown_Doc",
			ConfidenceScore: 0.0,
		},
		Metadata: metadata,
	}, fmt.Errorf("failed to get valid structured output after %d retries using model %s: %w", maxRetries, modelName, lastErr)
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
func (e *MLXEngine) MapReduceSummarize(ctx context.Context, text string, limit int, modelName, apiURL string) (string, error) {
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
		summary, err := e.summarizeChunk(ctx, chunk, i+1, len(chunks), modelName, apiURL)
		if err != nil {
			return "", fmt.Errorf("failed to summarize chunk %d/%d: %w", i+1, len(chunks), err)
		}
		summaries = append(summaries, summary)
	}

	// 3. Reduce: Combined and recursively summarize if needed
	combined := strings.Join(summaries, "\n\n")
	combinedTokens := e.ctxMgr.tokenizer.CountTokens(combined)

	if combinedTokens > limit {
		// Recursive reduction
		return e.MapReduceSummarize(ctx, combined, limit, modelName, apiURL)
	}

	return combined, nil
}

func (e *MLXEngine) summarizeChunk(ctx context.Context, text string, index, total int, modelName, apiURL string) (string, error) {
	// Ensure URL is set for the client
	if net, ok := e.llm.(*NetLLMClient); ok {
		fullURL := strings.TrimRight(apiURL, "/")
		if !strings.HasSuffix(fullURL, "/chat/completions") {
			fullURL += "/chat/completions"
		}
		net.apiURL = fullURL
	}

	prompt := fmt.Sprintf("Summarize the following document part (%d/%d). Keep key technical details, names, and core topics relevant for categorization:\n\n%s", index, total, text)

	reqBody := chatRequest{
		Model: modelName,
		Messages: []message{
			{Role: "system", Content: "You are a concise summarization assistant. Provide a brief but information-dense summary."},
			{Role: "user", Content: prompt},
		},
		Stream:      false,
		Temperature: 0.1,
	}

	chatResp, err := e.llm.CreateChatCompletion(ctx, reqBody)
	if err != nil {
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
