package mcpwire

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// HTTPConfig describes a Streamable HTTP server. The deprecated HTTP+SSE
// transport is unsupported (methodology 1.2).
type HTTPConfig struct {
	Endpoint string
	Headers  map[string]string
	Client   *http.Client
}

type httpTransport struct {
	cfg HTTPConfig

	mu        sync.Mutex
	revision  string
	sessionID string
	lastNote  string
}

// AuthError marks an HTTP status that maps to the auth failure class.
type AuthError struct {
	Status int
	Body   string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("http %d: %s", e.Status, truncate(e.Body, 400))
}

// DialHTTP builds a Streamable HTTP transport. No request is made until Call.
func DialHTTP(cfg HTTPConfig) Transport {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	return &httpTransport{cfg: cfg, revision: Rev20260728}
}

func (t *httpTransport) SetProtocolVersion(rev string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.revision = rev
}

func (t *httpTransport) Diagnostics() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastNote
}

func (t *httpTransport) Notify(ctx context.Context, payload []byte) error {
	_, err := t.post(ctx, payload, true)
	return err
}

func (t *httpTransport) Call(ctx context.Context, id int64, payload []byte) ([]byte, error) {
	return t.post(ctx, payload, false)
}

func (t *httpTransport) post(ctx context.Context, payload []byte, notification bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	t.mu.Lock()
	req.Header.Set("MCP-Protocol-Version", t.revision)
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()
	for k, v := range t.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := t.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if readErr != nil {
		return nil, readErr
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusPaymentRequired:
		return nil, &AuthError{Status: resp.StatusCode, Body: string(body)}
	case resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent:
		if notification {
			return nil, nil
		}
		return nil, fmt.Errorf("http %d with no response body", resp.StatusCode)
	case resp.StatusCode >= 400:
		t.mu.Lock()
		t.lastNote = truncate(string(body), 400)
		t.mu.Unlock()
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 400))
	}

	if notification {
		return nil, nil
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return extractSSEResponse(body)
	}
	return body, nil
}

// extractSSEResponse pulls the first JSON-RPC response frame out of an SSE
// body. Streamable HTTP may answer a POST with a stream whose events carry the
// response, so the raw bytes recorded are the event payload, not the framing.
func extractSSEResponse(body []byte) ([]byte, error) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64<<10), 64<<20)
	var data []string
	flush := func() ([]byte, bool) {
		if len(data) == 0 {
			return nil, false
		}
		joined := strings.Join(data, "\n")
		data = nil
		trimmed := strings.TrimSpace(joined)
		if trimmed == "" || trimmed[0] != '{' {
			return nil, false
		}
		var probe map[string]json.RawMessage
		if json.Unmarshal([]byte(trimmed), &probe) != nil {
			return nil, false
		}
		if _, ok := probe["id"]; !ok {
			return nil, false
		}
		return []byte(trimmed), true
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if out, ok := flush(); ok {
				return out, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if out, ok := flush(); ok {
		return out, nil
	}
	return nil, fmt.Errorf("no JSON-RPC response frame in event stream")
}

func (t *httpTransport) Close() error { return nil }

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
