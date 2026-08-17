package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultClaudeModel is the pinned model ID from methodology 1.6. A count
// without its model stamp is meaningless, so the pin travels with every cell.
const DefaultClaudeModel = "claude-opus-5"

const claudeEndpoint = "https://api.anthropic.com/v1/messages/count_tokens"

// AnthropicVersion is the API version header value.
const AnthropicVersion = "2023-06-01"

// Claude counts through the Anthropic count_tokens endpoint. No offline
// tokenizer exists for it.
type Claude struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

// AnthropicTool is one entry of the native tools parameter.
type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// NewClaude reads ANTHROPIC_API_KEY. Model may be empty to take the pin.
func NewClaude(model string) *Claude {
	if model == "" {
		model = DefaultClaudeModel
	}
	return &Claude{
		APIKey: os.Getenv("ANTHROPIC_API_KEY"),
		Model:  model,
		HTTP:   &http.Client{Timeout: 60 * time.Second},
	}
}

// Available reports whether a credential is configured.
func (c *Claude) Available() bool { return c.APIKey != "" }

// Count returns the token count of the canonical serialization carried as
// message content.
func (c *Claude) Count(ctx context.Context, text string) (int, error) {
	if !c.Available() {
		return 0, ErrUnavailable
	}
	body := map[string]any{
		"model":    c.Model,
		"messages": []any{map[string]any{"role": "user", "content": text}},
	}
	return c.post(ctx, body)
}

// CountNativeTools returns the count when the surface is mapped into Anthropic
// tool-definition shape and passed in the tools parameter. Methodology 1.5
// keeps this as a separate labelled figure, never blended into the
// cross-provider number.
func (c *Claude) CountNativeTools(ctx context.Context, tools []AnthropicTool) (int, error) {
	if !c.Available() {
		return 0, ErrUnavailable
	}
	if len(tools) == 0 {
		return 0, nil
	}
	body := map[string]any{
		"model":    c.Model,
		"messages": []any{map[string]any{"role": "user", "content": "."}},
		"tools":    tools,
	}
	return c.post(ctx, body)
}

func (c *Claude) post(ctx context.Context, body map[string]any) (int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	raw, err := retryPost(ctx, c.HTTP, claudeEndpoint, payload, map[string]string{
		"content-type":      "application/json",
		"x-api-key":         c.APIKey,
		"anthropic-version": AnthropicVersion,
	}, "count_tokens")
	if err != nil {
		return 0, err
	}
	var parsed struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("count_tokens response: %w", err)
	}
	return parsed.InputTokens, nil
}

// Stamp returns the cell provenance for a Claude count.
func (c *Claude) Stamp() Stamp {
	return Stamp{Model: c.Model, MeasuredAt: nowUTC()}
}
