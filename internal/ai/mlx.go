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
	apiURL    string
	modelName string
	client    *http.Client
}

// AnalysisResult is the structure we expect from the LLM.
type AnalysisResult struct {
	Category string `json:"category"`
	Title    string `json:"title"`
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
// modelName is the model identifier to send in the request (e.g. "mlx-community/Llama-3.2-1B-Instruct-4bit")
func NewMLXEngine(apiURL, modelName string) (*MLXEngine, error) {
	if apiURL == "" {
		apiURL = "http://localhost:8080/v1"
	}
	// Ensure URL ends with /chat/completions if not provided, or handle base URL.
	// Standard OpenAI SDKs take base URL. Here we'll just construct the full endpoint.
	if !strings.HasSuffix(apiURL, "/chat/completions") {
		apiURL = strings.TrimRight(apiURL, "/") + "/chat/completions"
	}

	return &MLXEngine{
		apiURL:    apiURL,
		modelName: modelName,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Categorize analyzes the text and returns a folder category and cleaned filename.
// Categorize analyzes the text and returns a folder category and cleaned filename.
func (e *MLXEngine) Categorize(ctx context.Context, text string) (*AnalysisResult, error) {
	systemPrompt := `You are a file organization assistant for Computer Science documents. Analyze the document text and return a SINGLE JSON object.
Required format: {"category": "Specific_Category_Name", "title": "Clean_Filename_No_Ext"}
Categories examples: Algorithms, Systems, AI, Math, Python, Cpp, Web, Data_Structures, Interviews, Research, Networking, Database, Security.
Do NOT return a list. Do NOT return markdown. Do NOT return extra text.`

	userPrompt := fmt.Sprintf("Document text snippet:\n%s", text)

	reqBody := chatRequest{
		Model: e.modelName,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream:      false,
		Temperature: 0.1, // Low temperature for deterministic output
	}

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

	// Log raw response for debugging
	// fmt.Printf("[DEBUG] Raw AI Response: %s\n", content)

	content = cleanJSON(content)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from model output: %w. Cleaned output: %s", err, content)
	}

	result.Category = SanitizeFilename(result.Category)
	result.Title = SanitizeFilename(result.Title)

	// Fallback/Safety for empty values
	if result.Category == "" {
		result.Category = "Misc"
	}
	if result.Title == "" {
		result.Title = "Unknown_Doc"
	}

	return &result, nil
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
