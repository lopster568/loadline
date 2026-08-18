package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultGeminiModel is the harness default. Methodology 1.6 records the
// Gemini model ID per run and leaves the v1 pin open, so this is overridable.
const DefaultGeminiModel = "gemini-3.1-pro-preview"

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

	raw, err := retryPost(ctx, g.HTTP, url, payload, map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": g.APIKey,
	}, "countTokens")
	if err != nil {
		return 0, err
	}
	var parsed struct {
		TotalTokens int `json:"totalTokens"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("countTokens response: %w", err)
	}
	return parsed.TotalTokens, nil
}

// Stamp returns the cell provenance for a Gemini count.
func (g *Gemini) Stamp() Stamp {
	return Stamp{Model: g.Model, MeasuredAt: nowUTC()}
}
