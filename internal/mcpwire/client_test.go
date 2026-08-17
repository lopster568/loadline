package mcpwire

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type scriptedTransport struct {
	handle   func(method string, params json.RawMessage) (any, *RPCError, error)
	notified []string
	revs     []string
}

func (s *scriptedTransport) Call(ctx context.Context, id int64, payload []byte) ([]byte, error) {
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	result, rpcErr, transportErr := s.handle(req.Method, req.Params)
	if transportErr != nil {
		return nil, transportErr
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	return json.Marshal(resp)
}

func (s *scriptedTransport) Notify(ctx context.Context, payload []byte) error {
	var req struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}
	s.notified = append(s.notified, req.Method)
	return nil
}

func (s *scriptedTransport) SetProtocolVersion(rev string) { s.revs = append(s.revs, rev) }
func (s *scriptedTransport) Diagnostics() string           { return "" }
func (s *scriptedTransport) Close() error                  { return nil }

func testClient(tr Transport) *Client {
	return NewClient(tr, ClientInfo{Name: "loadline-test", Version: "0.0.0"})
}

func TestNegotiateModernDiscover(t *testing.T) {
	tr := &scriptedTransport{handle: func(method string, params json.RawMessage) (any, *RPCError, error) {
		if method != "server/discover" {
			t.Fatalf("unexpected method %s", method)
		}
		var p struct {
			Meta map[string]any `json:"_meta"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			t.Fatalf("params: %v", err)
		}
		if p.Meta[MetaProtocolVersion] != Rev20260728 {
			t.Fatalf("modern request did not declare its version: %v", p.Meta)
		}
		if _, ok := p.Meta[MetaClientInfo]; !ok {
			t.Fatal("modern request did not carry clientInfo")
		}
		return map[string]any{
			"supportedVersions": []string{Rev20250618, Rev20260728},
			"capabilities":      map[string]any{},
			"_meta": map[string]any{
				MetaServerInfo: map[string]any{"name": "fake", "version": "1.2.3"},
			},
		}, nil, nil
	}}

	neg, err := testClient(tr).Negotiate(context.Background())
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if neg.Revision != Rev20260728 {
		t.Errorf("revision = %q, want %q", neg.Revision, Rev20260728)
	}
	if neg.Branch != BranchDiscover {
		t.Errorf("branch = %q, want %q", neg.Branch, BranchDiscover)
	}
	if neg.ServerInfo.Version != "1.2.3" {
		t.Errorf("serverInfo.version = %q", neg.ServerInfo.Version)
	}
}

func TestNegotiateUnsupportedVersionRetry(t *testing.T) {
	calls := 0
	var secondRev string
	tr := &scriptedTransport{handle: func(method string, params json.RawMessage) (any, *RPCError, error) {
		calls++
		if calls == 1 {
			return nil, &RPCError{
				Code:    CodeUnsupportedProtocolVersion,
				Message: "unsupported protocol version",
				Data:    json.RawMessage(`{"supported":["2025-06-18"]}`),
			}, nil
		}
		var p struct {
			Meta map[string]any `json:"_meta"`
		}
		_ = json.Unmarshal(params, &p)
		secondRev, _ = p.Meta[MetaProtocolVersion].(string)
		return map[string]any{
			"supportedVersions": []string{Rev20250618},
			"_meta":             map[string]any{MetaServerInfo: map[string]any{"name": "fake", "version": "9"}},
		}, nil, nil
	}}

	neg, err := testClient(tr).Negotiate(context.Background())
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if calls != 2 {
		t.Fatalf("discover called %d times, want 2", calls)
	}
	if secondRev != Rev20250618 {
		t.Errorf("retry declared %q, want %q", secondRev, Rev20250618)
	}
	if neg.Branch != BranchDiscoverRetry {
		t.Errorf("branch = %q, want %q", neg.Branch, BranchDiscoverRetry)
	}
	if neg.Revision != Rev20250618 {
		t.Errorf("revision = %q", neg.Revision)
	}
}

func TestNegotiateFallsBackToInitialize(t *testing.T) {
	for name, discoverFail := range map[string]func() (any, *RPCError, error){
		"method_not_found": func() (any, *RPCError, error) {
			return nil, &RPCError{Code: CodeMethodNotFound, Message: "Method not found"}, nil
		},
		"transport_error": func() (any, *RPCError, error) {
			return nil, nil, errors.New("unexpected frame")
		},
		"malformed_error": func() (any, *RPCError, error) {
			return nil, &RPCError{Code: -32600, Message: "Invalid Request"}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			fail := discoverFail
			tr := &scriptedTransport{}
			tr.handle = func(method string, params json.RawMessage) (any, *RPCError, error) {
				switch method {
				case "server/discover":
					return fail()
				case "initialize":
					var p struct {
						ProtocolVersion string `json:"protocolVersion"`
					}
					if err := json.Unmarshal(params, &p); err != nil {
						t.Fatalf("initialize params: %v", err)
					}
					if p.ProtocolVersion != Rev20250618 {
						t.Fatalf("first initialize asked for %q", p.ProtocolVersion)
					}
					return map[string]any{
						"protocolVersion": Rev20250618,
						"capabilities":    map[string]any{},
						"serverInfo":      map[string]any{"name": "legacy", "version": "0.4.1"},
					}, nil, nil
				}
				t.Fatalf("unexpected method %s", method)
				return nil, nil, nil
			}

			neg, err := testClient(tr).Negotiate(context.Background())
			if err != nil {
				t.Fatalf("negotiate: %v", err)
			}
			if neg.Branch != BranchLegacyHandshake {
				t.Errorf("branch = %q, want %q", neg.Branch, BranchLegacyHandshake)
			}
			if neg.Revision != Rev20250618 {
				t.Errorf("revision = %q", neg.Revision)
			}
			if neg.ServerInfo.Version != "0.4.1" {
				t.Errorf("serverInfo.version = %q", neg.ServerInfo.Version)
			}
			if len(tr.notified) != 1 || tr.notified[0] != "notifications/initialized" {
				t.Errorf("notifications = %v", tr.notified)
			}
		})
	}
}

func TestNegotiateLegacyDowngradesTo20250326(t *testing.T) {
	var asked []string
	tr := &scriptedTransport{}
	tr.handle = func(method string, params json.RawMessage) (any, *RPCError, error) {
		if method == "server/discover" {
			return nil, &RPCError{Code: CodeMethodNotFound, Message: "unknown"}, nil
		}
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(params, &p)
		asked = append(asked, p.ProtocolVersion)
		if p.ProtocolVersion == Rev20250618 {
			return nil, &RPCError{Code: -32602, Message: "unsupported protocolVersion"}, nil
		}
		return map[string]any{
			"protocolVersion": Rev20250326,
			"serverInfo":      map[string]any{"name": "old", "version": "0.1.0"},
		}, nil, nil
	}

	neg, err := testClient(tr).Negotiate(context.Background())
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if len(asked) != 2 || asked[1] != Rev20250326 {
		t.Fatalf("initialize attempts = %v", asked)
	}
	if neg.Revision != Rev20250326 {
		t.Errorf("revision = %q", neg.Revision)
	}
}

func TestNegotiateNoMutualRevision(t *testing.T) {
	tr := &scriptedTransport{handle: func(method string, params json.RawMessage) (any, *RPCError, error) {
		return map[string]any{"supportedVersions": []string{"2099-01-01"}}, nil, nil
	}}
	_, err := testClient(tr).Negotiate(context.Background())
	var protoErr *ProtocolError
	if !errors.As(err, &protoErr) {
		t.Fatalf("err = %v, want ProtocolError", err)
	}
}

func TestListToolsFollowsCursor(t *testing.T) {
	page := 0
	tr := &scriptedTransport{}
	tr.handle = func(method string, params json.RawMessage) (any, *RPCError, error) {
		if method == "server/discover" {
			return map[string]any{"supportedVersions": []string{Rev20260728}}, nil, nil
		}
		var p struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(params, &p)
		page++
		switch page {
		case 1:
			if p.Cursor != "" {
				t.Fatalf("first page sent cursor %q", p.Cursor)
			}
			return map[string]any{
				"tools":      []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}},
				"nextCursor": "page-2",
			}, nil, nil
		case 2:
			if p.Cursor != "page-2" {
				t.Fatalf("second page sent cursor %q", p.Cursor)
			}
			return map[string]any{"tools": []any{map[string]any{"name": "c"}}}, nil, nil
		}
		t.Fatalf("tools/list called %d times", page)
		return nil, nil, nil
	}

	c := testClient(tr)
	if _, err := c.Negotiate(context.Background()); err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	enum, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(enum.RawTools) != 3 {
		t.Errorf("tools = %d, want 3", len(enum.RawTools))
	}
	if len(enum.Pages) != 2 {
		t.Errorf("pages = %d, want 2", len(enum.Pages))
	}
	if enum.Partial {
		t.Error("enumeration reported partial")
	}
}

func TestListToolsPartialSurface(t *testing.T) {
	page := 0
	tr := &scriptedTransport{}
	tr.handle = func(method string, params json.RawMessage) (any, *RPCError, error) {
		if method == "server/discover" {
			return map[string]any{"supportedVersions": []string{Rev20260728}}, nil, nil
		}
		page++
		if page == 1 {
			return map[string]any{
				"tools":      []any{map[string]any{"name": "a"}},
				"nextCursor": "page-2",
			}, nil, nil
		}
		return nil, &RPCError{Code: -32000, Message: "cursor expired"}, nil
	}

	c := testClient(tr)
	if _, err := c.Negotiate(context.Background()); err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	enum, err := c.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !enum.Partial {
		t.Error("Partial = false, want true so the row fails as partial_surface")
	}
}

func TestCallRespectsContext(t *testing.T) {
	tr := &scriptedTransport{handle: func(method string, params json.RawMessage) (any, *RPCError, error) {
		return nil, nil, context.DeadlineExceeded
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := testClient(tr).Negotiate(ctx); err == nil {
		t.Fatal("expected an error")
	}
}

// A literal JSON null result is four bytes on the wire, so it clears a
// len(result) == 0 guard and then unmarshals into any result shape as a silent
// no-op. On tools/list that would publish an ok row with tool_count 0.
func TestNullResultIsAProtocolError(t *testing.T) {
	t.Run("tools_list", func(t *testing.T) {
		tr := &scriptedTransport{handle: func(method string, _ json.RawMessage) (any, *RPCError, error) {
			switch method {
			case "server/discover":
				return nil, &RPCError{Code: CodeMethodNotFound, Message: "Method not found"}, nil
			case "initialize":
				return map[string]any{
					"protocolVersion": Rev20250618,
					"serverInfo":      map[string]any{"name": "null-result", "version": "1.0.0"},
				}, nil, nil
			}
			// tools/list replies {"result": null}.
			return nil, nil, nil
		}}
		c := testClient(tr)
		if _, err := c.Negotiate(context.Background()); err != nil {
			t.Fatalf("negotiate: %v", err)
		}
		enum, err := c.ListTools(context.Background())
		if err == nil {
			t.Fatalf("a null tools/list result was accepted: %+v", enum)
		}
		var protoErr *ProtocolError
		if !errors.As(err, &protoErr) {
			t.Fatalf("err = %T (%v), want a protocol error", err, err)
		}
		if !strings.Contains(err.Error(), "null result") {
			t.Errorf("error does not name the cause: %v", err)
		}
		if enum != nil && len(enum.RawTools) != 0 {
			t.Errorf("tools were harvested from a null result: %v", enum.RawTools)
		}
	})

	t.Run("discover_and_initialize", func(t *testing.T) {
		// Every method replies {"result": null}. server/discover falling through
		// to the legacy probe is methodology 1.3 behaviour; initialize doing the
		// same must not then produce an empty negotiation.
		tr := &scriptedTransport{handle: func(string, json.RawMessage) (any, *RPCError, error) {
			return nil, nil, nil
		}}
		neg, err := testClient(tr).Negotiate(context.Background())
		if err == nil {
			t.Fatalf("a null handshake result was accepted: %+v", neg)
		}
		var protoErr *ProtocolError
		if !errors.As(err, &protoErr) {
			t.Fatalf("err = %T (%v), want a protocol error", err, err)
		}
	})
}
