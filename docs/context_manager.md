# AI Context Manager

The Context Manager is responsible for ensuring that all interactions with the LLM (Large Language Model) stay within technical constraints (context windows) while maximizing the quality of the prompt.

## Overview

LLMs have a finite "context window" (the default is 4,096 tokens). This window is configurable via the `-ctx` flag, `ctx` YAML key, or `DOCS_CTX` environment variable. If a document is too large to fit in this window, the API request will fail. The Context Manager prevents this by intelligently truncating content and protecting critical instructions.

## Components

### 1. Tokenizer (`tokenizer.go`)
Uses `tiktoken-go` to accurately count tokens. 
- **Default Encoding**: `cl100k_base` (optimized for GPT-4 and Llama 3 models).
- **Fallback**: Automatically falls back to standard encoding if the specific model mapping is missing.

### 2. Budget Allocation
To ensure the model always has enough room to "breathe" and remember its instructions, the manager divides the token pool:

| Section | Budget | Purpose |
| :--- | :--- | :--- |
| **System Prompt** | 10% | Protects instructions and personas. |
| **Examples** | 20% | Reserves space for few-shot examples (future-proof). |
| **Document Content** | 60% | The primary area for variable input (document text). |
| **Output Reserve** | 10% | Ensures the model isn't cut off mid-response. |

### 3. Truncation Strategies

When a document exceeds its 60% budget, the manager applies one of the following strategies:

#### Sliding Window (Head + Tail)
Keeps the beginning and the end of the text. This is useful for logs or chronological data where the start and end define the scope.

#### Middle Extraction (Industry Standard)
Removes the middle segment of the text. Based on research (e.g., "Lost in the Middle"), LLMs are most effective at utilizing context at the very beginning (introductions) and very end (conclusions) of a prompt.

#### Map-Reduce Summarization (Advanced Fallback)
For documents that significantly exceed the context window, the engine uses a Map-Reduce approach:
1. **Map**: The document is split into token-aware chunks. Each chunk is summarized by the AI in parallel.
2. **Reduce**: The resulting summaries are combined. If the combination still exceeds the limit, the process repeats recursively until a final information-dense summary is produced.
3. **Usage**: This is automatically triggered as a fallback if a simple extraction would lose too much context.

## Integration

The `MLXEngine` uses the `ContextManager` automatically in its `Categorize` method:

```go
// Inside Categorize
systemPrompt = e.ctxMgr.Truncate(systemPrompt, systemBudget, StrategySlidingWindow)
text = e.ctxMgr.Truncate(text, contentBudget, StrategyMiddleExtraction)
```

## Testing

Comprehensive tests are located in:
- `internal/ai/tokenizer_test.go`
- `internal/ai/context_manager_test.go`

To run the AI suite:
```bash
go test ./internal/ai/...
```
