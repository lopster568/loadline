package mcpwire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
)

// ClientInfo is the honest identity the harness sends. Methodology 8 commits
// the harness to identifying itself rather than spoofing a generic client.
type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

// Branch names the path taken through the revision probe in methodology 1.3.
const (
	BranchDiscover        = "discover"
	BranchDiscoverRetry   = "discover_version_retry"
	BranchLegacyHandshake = "legacy_initialize"
)

// Negotiation records what the revision probe established.
type Negotiation struct {
	Revision          string     `json:"revision"`
	Branch            string     `json:"branch"`
	SupportedVersions []string   `json:"supported_versions,omitempty"`
	ServerInfo        ServerInfo `json:"server_info"`
}

// Client speaks JSON-RPC 2.0 MCP over one transport.
type Client struct {
	tr         Transport
	info       ClientInfo
	nextID     atomic.Int64
	modern     bool
	negotiated Negotiation
}

// NewClient wraps a transport. Negotiate must be called before ListTools.
func NewClient(tr Transport, info ClientInfo) *Client {
	return &Client{tr: tr, info: info}
}

// Negotiated returns the recorded negotiation outcome.
func (c *Client) Negotiated() Negotiation { return c.negotiated }

// Close releases the transport.
func (c *Client) Close() error { return c.tr.Close() }

// Diagnostics returns transport-side failure context.
func (c *Client) Diagnostics() string { return c.tr.Diagnostics() }

// ProtocolError marks an exchange that reached the server but could not
// complete, mapping to the protocol_error failure class.
type ProtocolError struct{ Detail string }

func (e *ProtocolError) Error() string { return e.Detail }

// Negotiate runs the methodology 1.3 probe: server/discover first, then the
// version retry on -32022, then the legacy initialize fallback.
func (c *Client) Negotiate(ctx context.Context) (Negotiation, error) {
	c.tr.SetProtocolVersion(Rev20260728)
	res, rpcErr, err := c.call(ctx, "server/discover", c.modernParams(Rev20260728, nil))

	switch {
	case err == nil && rpcErr == nil:
		return c.finishDiscover(res, Rev20260728, BranchDiscover)

	case rpcErr != nil && rpcErr.Code == CodeUnsupportedProtocolVersion:
		supported := supportedFromErrorData(rpcErr.Data)
		pick := highestMutual(supported)
		if pick == "" {
			return Negotiation{}, &ProtocolError{Detail: fmt.Sprintf(
				"no mutually supported revision; server offers %v", supported)}
		}
		c.tr.SetProtocolVersion(pick)
		res2, rpcErr2, err2 := c.call(ctx, "server/discover", c.modernParams(pick, nil))
		if err2 != nil {
			return Negotiation{}, err2
		}
		if rpcErr2 != nil {
			return Negotiation{}, &ProtocolError{Detail: rpcErr2.Error()}
		}
		return c.finishDiscover(res2, pick, BranchDiscoverRetry)

	default:
		// Any other error or a timeout is treated as a legacy server, matching
		// the specification's stdio backward-compatibility probe.
		var authErr *AuthError
		if errors.As(err, &authErr) {
			return Negotiation{}, err
		}
		if ctx.Err() != nil {
			return Negotiation{}, ctx.Err()
		}
		return c.legacyHandshake(ctx)
	}
}

func (c *Client) finishDiscover(res json.RawMessage, probed, branch string) (Negotiation, error) {
	var parsed struct {
		SupportedVersions []string                   `json:"supportedVersions"`
		Capabilities      json.RawMessage            `json:"capabilities"`
		Meta              map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(res, &parsed); err != nil {
		return Negotiation{}, &ProtocolError{Detail: fmt.Sprintf("malformed DiscoverResult: %v", err)}
	}
	pick := probed
	if len(parsed.SupportedVersions) > 0 {
		if m := highestMutual(parsed.SupportedVersions); m != "" {
			pick = m
		} else {
			return Negotiation{}, &ProtocolError{Detail: fmt.Sprintf(
				"no mutually supported revision; server offers %v", parsed.SupportedVersions)}
		}
	}
	var info ServerInfo
	if raw, ok := parsed.Meta[MetaServerInfo]; ok {
		_ = json.Unmarshal(raw, &info)
	}
	c.modern = true
	c.tr.SetProtocolVersion(pick)
	c.negotiated = Negotiation{
		Revision:          pick,
		Branch:            branch,
		SupportedVersions: parsed.SupportedVersions,
		ServerInfo:        info,
	}
	return c.negotiated, nil
}

func (c *Client) legacyHandshake(ctx context.Context) (Negotiation, error) {
	var lastErr error
	for _, want := range []string{Rev20250618, Rev20250326} {
		c.tr.SetProtocolVersion(want)
		params := map[string]any{
			"protocolVersion": want,
			"capabilities":    map[string]any{},
			"clientInfo":      c.info,
		}
		res, rpcErr, err := c.call(ctx, "initialize", params)
		if err != nil {
			lastErr = err
			var authErr *AuthError
			if errors.As(err, &authErr) || ctx.Err() != nil {
				return Negotiation{}, err
			}
			continue
		}
		if rpcErr != nil {
			lastErr = rpcErr
			continue
		}
		var parsed struct {
			ProtocolVersion string     `json:"protocolVersion"`
			ServerInfo      ServerInfo `json:"serverInfo"`
		}
		if err := json.Unmarshal(res, &parsed); err != nil {
			return Negotiation{}, &ProtocolError{Detail: fmt.Sprintf("malformed InitializeResult: %v", err)}
		}
		if !known(parsed.ProtocolVersion) {
			lastErr = &ProtocolError{Detail: fmt.Sprintf(
				"server selected unsupported revision %q", parsed.ProtocolVersion)}
			continue
		}
		c.tr.SetProtocolVersion(parsed.ProtocolVersion)
		_ = c.notify(ctx, "notifications/initialized", map[string]any{})
		c.negotiated = Negotiation{
			Revision:   parsed.ProtocolVersion,
			Branch:     BranchLegacyHandshake,
			ServerInfo: parsed.ServerInfo,
		}
		return c.negotiated, nil
	}
	if lastErr == nil {
		lastErr = &ProtocolError{Detail: "initialize produced no usable result"}
	}
	return Negotiation{}, lastErr
}

// Enumeration is the result of a full tools/list sweep.
type Enumeration struct {
	Pages    [][]byte
	RawTools [][]byte
	Partial  bool
	Err      error
}

// MaxPages bounds cursor following so a broken server cannot loop forever.
const MaxPages = 200

// ListTools issues tools/list and follows nextCursor to exhaustion
// (methodology 1.4). A partial enumeration is returned with Partial set, which
// the caller maps to the partial_surface failure class.
func (c *Client) ListTools(ctx context.Context) (*Enumeration, error) {
	out := &Enumeration{}
	cursor := ""
	for page := 0; page < MaxPages; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if c.modern {
			params = c.modernParams(c.negotiated.Revision, params)
		}
		raw, rpcErr, err := c.callRaw(ctx, "tools/list", params)
		if err != nil {
			out.Partial = page > 0
			out.Err = err
			return out, err
		}
		if rpcErr != nil {
			out.Partial = page > 0
			out.Err = rpcErr
			return out, rpcErr
		}
		out.Pages = append(out.Pages, raw.wire)

		var parsed struct {
			Tools      []json.RawMessage `json:"tools"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw.result, &parsed); err != nil {
			out.Partial = true
			out.Err = &ProtocolError{Detail: fmt.Sprintf("malformed ListToolsResult: %v", err)}
			return out, out.Err
		}
		for _, t := range parsed.Tools {
			out.RawTools = append(out.RawTools, []byte(t))
		}
		if parsed.NextCursor == "" {
			return out, nil
		}
		cursor = parsed.NextCursor
	}
	out.Partial = true
	out.Err = &ProtocolError{Detail: fmt.Sprintf("pagination exceeded %d pages", MaxPages)}
	return out, out.Err
}

func (c *Client) modernParams(rev string, base map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	base["_meta"] = map[string]any{
		MetaProtocolVersion: rev,
		MetaClientInfo:      c.info,
	}
	return base
}

type rawCall struct {
	wire   []byte
	result json.RawMessage
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, *RPCError, error) {
	r, rpcErr, err := c.callRaw(ctx, method, params)
	if r == nil {
		return nil, rpcErr, err
	}
	return r.result, rpcErr, err
}

func (c *Client) callRaw(ctx context.Context, method string, params any) (*rawCall, *RPCError, error) {
	id := c.nextID.Add(1)
	payload, err := json.Marshal(Request{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return nil, nil, err
	}
	wire, err := c.tr.Call(ctx, id, payload)
	if err != nil {
		return nil, nil, err
	}
	var resp Response
	if err := json.Unmarshal(wire, &resp); err != nil {
		return nil, nil, &ProtocolError{Detail: fmt.Sprintf("malformed response to %s: %v", method, err)}
	}
	if resp.Error != nil {
		return &rawCall{wire: wire}, resp.Error, nil
	}
	// A literal JSON null is four bytes, so it clears a length check while
	// unmarshalling into any result shape as a silent no-op. On tools/list that
	// would publish an ok row with zero tools; on server/discover or initialize
	// it would publish an empty negotiation. JSON-RPC 2.0 requires exactly one
	// of result or error, and none of the MCP methods the harness calls has a
	// null result, so both cases are protocol errors.
	result := bytes.TrimSpace(resp.Result)
	if len(result) == 0 {
		return nil, nil, &ProtocolError{Detail: fmt.Sprintf("response to %s carries neither result nor error", method)}
	}
	if bytes.Equal(result, []byte("null")) {
		return nil, nil, &ProtocolError{Detail: fmt.Sprintf("response to %s carries a null result, which is neither a result nor an error", method)}
	}
	return &rawCall{wire: wire, result: resp.Result}, nil, nil
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	payload, err := json.Marshal(Request{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	return c.tr.Notify(ctx, payload)
}

func supportedFromErrorData(data json.RawMessage) []string {
	if len(data) == 0 {
		return nil
	}
	var parsed struct {
		Supported []string `json:"supported"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil
	}
	return parsed.Supported
}

// highestMutual picks the highest revision present in both the server list and
// the harness set. Methodology 1.3 measures at the highest mutual revision only.
func highestMutual(serverVersions []string) string {
	for _, ours := range HarnessRevisions {
		for _, theirs := range serverVersions {
			if ours == theirs {
				return ours
			}
		}
	}
	return ""
}

func known(rev string) bool {
	for _, r := range HarnessRevisions {
		if r == rev {
			return true
		}
	}
	return false
}
