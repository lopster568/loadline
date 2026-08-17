package mcpwire

import (
	"context"
	"encoding/json"
	"fmt"
)

// Protocol revisions the harness speaks, highest first (methodology 1.3).
const (
	Rev20260728 = "2026-07-28"
	Rev20250618 = "2025-06-18"
	Rev20250326 = "2025-03-26"
)

// HarnessRevisions is the harness revision set, ordered highest to lowest.
var HarnessRevisions = []string{Rev20260728, Rev20250618, Rev20250326}

// MetaProtocolVersion and MetaClientInfo are the 2026-07-28 _meta keys.
const (
	MetaProtocolVersion = "io.modelcontextprotocol/protocolVersion"
	MetaClientInfo      = "io.modelcontextprotocol/clientInfo"
	MetaServerInfo      = "io.modelcontextprotocol/serverInfo"
)

// CodeUnsupportedProtocolVersion is UnsupportedProtocolVersionError.
const CodeUnsupportedProtocolVersion = -32022

// CodeMethodNotFound is the JSON-RPC method-not-found code.
const CodeMethodNotFound = -32601

// Request is an outbound JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response is an inbound JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// Transport carries JSON-RPC frames to one server.
type Transport interface {
	// Call sends a request and returns the raw response bytes as received.
	Call(ctx context.Context, id int64, payload []byte) ([]byte, error)
	// Notify sends a notification, which has no response.
	Notify(ctx context.Context, payload []byte) error
	// SetProtocolVersion records the negotiated revision for transports that
	// must echo it on later requests.
	SetProtocolVersion(rev string)
	// Diagnostics returns transport-side context for a failure report.
	Diagnostics() string
	Close() error
}

// ServerInfo is the self-reported server identity. The protocol does not
// verify it (methodology 1.1).
type ServerInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}
