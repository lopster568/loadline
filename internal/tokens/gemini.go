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

// DefaultGeminiModel is the harness default. Methodology 1.6 records the
// Gemini model ID per run and leaves the v1 pin open, so this is overridable.
const DefaultGeminiModel = "gemini-2.5-pro"

const geminiBase = "https://generativelanguage.googleapis.com/v1beta/models/"

// Gemini counts through the countTokens REST endpoint.
type Gemini struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

// NewGemini reads GEMINI_API_KEY, falling back to GOOGLE_API_KEY.
func NewGemini(model string) *Gemini {
	if model == "" {
		model = DefaultGeminiModel
	}
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	return &Gemini{APIKey: key, Model: model, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Available reports whether a credential is configured.
func (g *Gemini) Available() bool { return g.APIKey != "" }

// Count returns the token count of the canonical serialization.
func (g *Gemini) Count(ctx context.Context, text string) (int, error) {
	if !g.Available() {
		return 0, ErrUnavailable
	}
	payload, err := json.Marshal(map[string]any{
		"contents": []any{map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"text": text}},
		}},
	})
	if err != nil {
		return 0, err
	}
	url := geminiBase + g.Model + ":countTokens"

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.APIKey)

		resp, err := g.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("countTokens http %d: %s", resp.StatusCode, trim(raw))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("countTokens http %d: %s", resp.StatusCode, trim(raw))
		}
		var parsed struct {
			TotalTokens int `json:"totalTokens"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return 0, fmt.Errorf("countTokens response: %w", err)
		}
		return parsed.TotalTokens, nil
	}
	return 0, lastErr
}

// Stamp returns the cell provenance for a Gemini count.
func (g *Gemini) Stamp() Stamp {
	return Stamp{Model: g.Model, MeasuredAt: nowUTC()}
}
