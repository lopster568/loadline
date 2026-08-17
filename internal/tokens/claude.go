package tokens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeEndpoint, bytes.NewReader(payload))
		if err != nil {
			return 0, err
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("x-api-key", c.APIKey)
		req.Header.Set("anthropic-version", AnthropicVersion)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("count_tokens http %d: %s", resp.StatusCode, trim(raw))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("count_tokens http %d: %s", resp.StatusCode, trim(raw))
		}
		var parsed struct {
			InputTokens int `json:"input_tokens"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return 0, fmt.Errorf("count_tokens response: %w", err)
		}
		return parsed.InputTokens, nil
	}
	return 0, lastErr
}

// Stamp returns the cell provenance for a Claude count.
func (c *Claude) Stamp() Stamp {
	return Stamp{Model: c.Model, MeasuredAt: nowUTC()}
}

func trim(b []byte) string {
	s := string(bytes.TrimSpace(b))
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}
