// Package tokens holds one adapter per provider tokenizer. An adapter that
// has no credential reports itself unavailable rather than estimating; a cell
// without a real count is never published (methodology 1.7).
package tokens

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnavailable is returned by an adapter with no usable credential.
var ErrUnavailable = errors.New("tokenizer unavailable: no credential configured")

// retryAttempts bounds the count_tokens retries. Both hosted tokenizers are
// free at sweep volumes, so the only thing a retry costs is wall clock.
const retryAttempts = 3

// retryPost issues one POST up to retryAttempts times, retrying a transport
// failure or a 429/5xx and failing immediately on any other non-200. label
// names the endpoint in the error text. It returns the response body.
func retryPost(ctx context.Context, hc *http.Client, url string, payload []byte, headers map[string]string, label string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%s http %d: %s", label, resp.StatusCode, trim(raw))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s http %d: %s", label, resp.StatusCode, trim(raw))
		}
		return raw, nil
	}
	return nil, lastErr
}

func trim(b []byte) string {
	s := string(bytes.TrimSpace(b))
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

// Stamp is the per-cell provenance required by methodology 1.7.
type Stamp struct {
	Model      string    `json:"model,omitempty"`
	Encoding   string    `json:"encoding,omitempty"`
	MeasuredAt time.Time `json:"measured_at"`
}

func nowUTC() time.Time { return time.Now().UTC() }
