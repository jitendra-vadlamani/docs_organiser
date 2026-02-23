package ai

// TruncationStrategy defines how to shorten text to fit within a token budget.
type TruncationStrategy string

const (
	StrategySlidingWindow    TruncationStrategy = "sliding_window"
	StrategyMiddleExtraction TruncationStrategy = "middle_extraction"
	StrategyMapReduce        TruncationStrategy = "map_reduce" // Placeholder for complex summarization
)

// ContextManager handles token budgeting and truncation logic.
type ContextManager struct {
	tokenizer *Tokenizer
	maxTokens int

	// Budget percentages
	systemBudgetPct   float64
	examplesBudgetPct float64
	contentBudgetPct  float64
	outputBudgetPct   float64
}

// NewContextManager creates a new ContextManager with default budget allocation.
func NewContextManager(tokenizer *Tokenizer, maxTokens int) *ContextManager {
	if maxTokens <= 0 {
		maxTokens = 4096 // Default safe limit for many models
	}
	return &ContextManager{
		tokenizer:         tokenizer,
		maxTokens:         maxTokens,
		systemBudgetPct:   0.10,
		examplesBudgetPct: 0.20,
		contentBudgetPct:  0.60,
		outputBudgetPct:   0.10,
	}
}

// GetBudgets returns token limits for each section.
func (cm *ContextManager) GetBudgets() (system, examples, content, output int) {
	system = int(float64(cm.maxTokens) * cm.systemBudgetPct)
	examples = int(float64(cm.maxTokens) * cm.examplesBudgetPct)
	content = int(float64(cm.maxTokens) * cm.contentBudgetPct)
	output = int(float64(cm.maxTokens) * cm.outputBudgetPct)
	return
}

// Truncate optimizes text to fit within the specified token limit using a strategy.
func (cm *ContextManager) Truncate(text string, limit int, strategy TruncationStrategy) string {
	tokens := cm.tokenizer.encoding.Encode(text, nil, nil)
	if len(tokens) <= limit {
		return text
	}

	switch strategy {
	case StrategyMiddleExtraction:
		return cm.middleExtraction(tokens, limit)
	case StrategySlidingWindow:
		fallthrough
	default:
		return cm.slidingWindow(tokens, limit)
	}
}

// slidingWindow keeps the beginning and end of the text (Head + Tail).
func (cm *ContextManager) slidingWindow(tokens []int, limit int) string {
	headSize := limit / 2
	tailSize := limit - headSize

	headTokens := tokens[:headSize]
	tailTokens := tokens[len(tokens)-tailSize:]

	headText := cm.tokenizer.encoding.Decode(headTokens)
	tailText := cm.tokenizer.encoding.Decode(tailTokens)

	return headText + "\n[... truncated ...]\n" + tailText
}

// middleExtraction removes the middle part of the text, keeping the most relevant context.
// In many documents, the beginning (intro) and end (conclusion) are most important.
func (cm *ContextManager) middleExtraction(tokens []int, limit int) string {
	// For middle extraction, we keep the first 30% and last 70% of the allowed tokens
	// or some other heuristic. Let's do 40/60.
	headSize := int(float64(limit) * 0.4)
	tailSize := limit - headSize

	headText := cm.tokenizer.encoding.Decode(tokens[:headSize])
	tailText := cm.tokenizer.encoding.Decode(tokens[len(tokens)-tailSize:])

	return headText + "\n[... content extracted ...]\n" + tailText
}

// EstimateResponseBudget returns the estimated tokens available for the response.
func (cm *ContextManager) EstimateResponseBudget() int {
	_, _, _, output := cm.GetBudgets()
	return output
}

// IsExceedingHardLimit checks if the total tokens exceed the hard maximum.
func (cm *ContextManager) IsExceedingHardLimit(text string) bool {
	return cm.tokenizer.CountTokens(text) > cm.maxTokens
}

// Chunk splits text into slices that each fit within chunkSize tokens.
func (cm *ContextManager) Chunk(text string, chunkSize int) []string {
	tokens := cm.tokenizer.encoding.Encode(text, nil, nil)
	if len(tokens) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(tokens); i += chunkSize {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		chunks = append(chunks, cm.tokenizer.encoding.Decode(tokens[i:end]))
	}
	return chunks
}
