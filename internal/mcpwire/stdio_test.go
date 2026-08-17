package mcpwire

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake stdio MCP server so DialStdio is exercised
// against a real subprocess rather than a mock.
const fakeServerEnv = "LOADLINE_FAKE_MCP_SERVER"

func TestMain(m *testing.M) {
	switch os.Getenv(fakeServerEnv) {
	case "":
		os.Exit(m.Run())
	case "legacy":
		runFakeServer(false)
	case "noisy-legacy":
		fmt.Println("starting fake server on stdout")
		fmt.Fprintln(os.Stderr, "log line to stderr")
		runFakeServer(false)
	case "modern":
		runFakeServer(true)
	case "crash":
		fmt.Fprintln(os.Stderr, "fatal: missing required argument")
		os.Exit(1)
	}
	os.Exit(0)
}

func runFakeServer(modern bool) {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), 8<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	page := 0
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Cursor string `json:"cursor"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(line), &req) != nil {
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
		switch req.Method {
		case "server/discover":
			if !modern {
				resp["error"] = map[string]any{"code": CodeMethodNotFound, "message": "Method not found"}
			} else {
				resp["result"] = map[string]any{
					"supportedVersions": []string{Rev20260728},
					"_meta": map[string]any{
						MetaServerInfo: map[string]any{"name": "fake-stdio", "version": "7.7.7"},
					},
				}
			}
		case "initialize":
			resp["result"] = map[string]any{
				"protocolVersion": Rev20250618,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fake-stdio", "version": "7.7.7"},
			}
		case "tools/list":
			page++
			if page == 1 {
				resp["result"] = map[string]any{
					"tools": []any{map[string]any{
						"name":        "read_file",
						"description": "Read a file",
						"inputSchema": map[string]any{"type": "object"},
					}},
					"nextCursor": "next",
				}
			} else {
				resp["result"] = map[string]any{
					"tools": []any{map[string]any{
						"name":        "write_file",
						"description": "Write a file",
						"inputSchema": map[string]any{"type": "object"},
					}},
				}
			}
		default:
			resp["error"] = map[string]any{"code": CodeMethodNotFound, "message": "Method not found"}
		}
		b, _ := json.Marshal(resp)
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
}

func dialFake(t *testing.T, mode string) Transport {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tr, err := DialStdio(ctx, StdioConfig{
		Command: os.Args[0],
		Env:     append(os.Environ(), fakeServerEnv+"="+mode),
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { tr.Close() })
	return tr
}

func TestStdioLegacyServerEndToEnd(t *testing.T) {
	for _, mode := range []string{"legacy", "noisy-legacy"} {
		t.Run(mode, func(t *testing.T) {
			c := testClient(dialFake(t, mode))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			neg, err := c.Negotiate(ctx)
			if err != nil {
				t.Fatalf("negotiate: %v", err)
			}
			if neg.Branch != BranchLegacyHandshake || neg.Revision != Rev20250618 {
				t.Fatalf("negotiation = %+v", neg)
			}
			enum, err := c.ListTools(ctx)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(enum.RawTools) != 2 || len(enum.Pages) != 2 {
				t.Fatalf("tools=%d pages=%d", len(enum.RawTools), len(enum.Pages))
			}
		})
	}
}

func TestStdioModernServerEndToEnd(t *testing.T) {
	c := testClient(dialFake(t, "modern"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	neg, err := c.Negotiate(ctx)
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if neg.Branch != BranchDiscover || neg.Revision != Rev20260728 {
		t.Fatalf("negotiation = %+v", neg)
	}
	if neg.ServerInfo.Version != "7.7.7" {
		t.Fatalf("serverInfo = %+v", neg.ServerInfo)
	}
}

func TestStdioServerThatExitsImmediately(t *testing.T) {
	c := testClient(dialFake(t, "crash"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := c.Negotiate(ctx); err == nil {
		t.Fatal("expected an error from a server that exits at startup")
	}
	if diag := c.Diagnostics(); !strings.Contains(diag, "missing required argument") {
		t.Errorf("stderr not captured for the failure row: %q", diag)
	}
}

func TestStdioMissingExecutable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := DialStdio(ctx, StdioConfig{Command: "loadline-no-such-binary-xyz"})
	if err == nil {
		t.Fatal("expected a launch failure")
	}
}
