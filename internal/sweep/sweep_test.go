package sweep

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lopster568/loadline/internal/corpus"
	"github.com/lopster568/loadline/internal/mcpwire"
	"github.com/lopster568/loadline/internal/report"
)

// fakeServerFlag turns the test binary into a stdio MCP server so the sweep is
// exercised end to end against a real subprocess.
const fakeServerFlag = "-loadline-fake-server"

func TestMain(m *testing.M) {
	for _, a := range os.Args[1:] {
		if a == fakeServerFlag {
			runFakeServer()
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func runFakeServer() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), 8<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal([]byte(line), &req) != nil || len(req.ID) == 0 {
			continue
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
		switch req.Method {
		case "server/discover":
			resp["error"] = map[string]any{"code": mcpwire.CodeMethodNotFound, "message": "Method not found"}
		case "initialize":
			resp["result"] = map[string]any{
				"protocolVersion": mcpwire.Rev20250618,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "sweep-fake", "version": "3.1.4"},
			}
		case "tools/list":
			resp["result"] = map[string]any{"tools": []any{
				map[string]any{
					"name":        "read_file",
					"description": "Read the contents of a file from the allowed directories.",
					"inputSchema": map[string]any{
						"type":       "object",
						"properties": map[string]any{"path": map[string]any{"type": "string", "description": "Absolute path to read"}},
						"required":   []string{"path"},
					},
				},
				map[string]any{
					"name":        "write_file",
					"description": "Write text to a file, creating it when absent.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path":     map[string]any{"type": "string", "description": "Absolute path to write"},
							"contents": map[string]any{"type": "string", "description": "Text to write"},
						},
						"required": []string{"path", "contents"},
					},
				},
			}}
		default:
			resp["error"] = map[string]any{"code": mcpwire.CodeMethodNotFound, "message": "Method not found"}
		}
		b, _ := json.Marshal(resp)
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
}

func ptrBool(b bool) *bool       { return &b }
func ptrString(s string) *string { return &s }

func fakeEntry() corpus.Server {
	return corpus.Server{
		ID:             "fake",
		Name:           "Fake stdio server",
		Category:       "dev-tools",
		MaintainerType: "official",
		Transport:      []string{"stdio"},
		Auth:           corpus.Auth{Required: ptrBool(false)},
		Package: map[string]corpus.Pkg{
			"stdio": {Type: "binary", Command: os.Args[0], Args: []string{fakeServerFlag}, Version: "3.1.4"},
		},
	}
}

func TestSweepEndToEndProducesMeasuredRow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StepTimeout = 20 * time.Second
	cfg.ServerTimeout = 40 * time.Second
	cfg.ScratchDir = t.TempDir()

	doc := NewRunner(cfg).Run(context.Background(), []corpus.Server{fakeEntry()})
	if len(doc.Servers) != 1 {
		t.Fatalf("rows = %d", len(doc.Servers))
	}
	row := doc.Servers[0]
	if row.Status != report.StatusOK {
		t.Fatalf("status = %s (%s)", row.Status, row.Error)
	}
	if row.ToolCount != 2 {
		t.Errorf("tool_count = %d, want 2", row.ToolCount)
	}
	if row.ProtocolRevision != mcpwire.Rev20250618 {
		t.Errorf("protocol_revision = %q", row.ProtocolRevision)
	}
	if row.Provenance.ServerVersion != "3.1.4" {
		t.Errorf("server_version = %q", row.Provenance.ServerVersion)
	}
	if row.Provenance.WireSHA256 == "" || row.Provenance.CanonicalSHA256 == "" {
		t.Error("provenance hashes missing")
	}
	if !row.Counts.OpenAI.Available || row.Counts.OpenAI.TotalSchemaTokens <= 0 {
		t.Fatalf("openai cell = %+v", row.Counts.OpenAI)
	}
	if len(row.Counts.OpenAI.PerTool) != 2 {
		t.Errorf("per_tool = %v", row.Counts.OpenAI.PerTool)
	}
	if row.Counts.OpenAI.PerTool["read_file"] <= 0 {
		t.Error("per-tool count is not positive")
	}
	if row.Modes == nil {
		t.Fatal("a fully counted row published no mode block")
	}
	if row.Modes.Naive.Kind != "measured" || row.Modes.Naive.Tokens != row.Counts.OpenAI.TotalSchemaTokens {
		t.Errorf("naive mode = %+v", row.Modes.Naive)
	}
	if row.Modes.ToolSearch.Kind != "modeled" || row.Modes.ToolSearch.PerToolAvg <= 0 {
		t.Errorf("tool_search mode = %+v", row.Modes.ToolSearch)
	}
	if row.Modes.CodeMode.Kind != "modeled" {
		t.Errorf("code_mode kind = %q", row.Modes.CodeMode.Kind)
	}
	if row.Acquisition == nil || row.Acquisition.Transport != "stdio" {
		t.Errorf("acquisition = %+v", row.Acquisition)
	}

	// A missing credential must leave the cell unavailable rather than faked.
	if os.Getenv("ANTHROPIC_API_KEY") == "" && row.Counts.Claude.Available {
		t.Error("claude cell reported available without a credential")
	}
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" && row.Counts.Gemini.Available {
		t.Error("gemini cell reported available without a credential")
	}

	out := t.TempDir()
	written, err := report.Write(out, doc)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("artifacts = %v", written)
	}
	perServer := filepath.Join(out, "runs", doc.Run.Date, "fake.json")
	if _, err := os.Stat(perServer); err != nil {
		t.Fatalf("per-server artifact: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "latest.json"))
	if err != nil {
		t.Fatalf("latest.json: %v", err)
	}
	var reread report.Document
	if err := json.Unmarshal(raw, &reread); err != nil {
		t.Fatalf("latest.json is not valid JSON: %v", err)
	}
	if reread.SchemaVersion != report.SchemaVersion || reread.Sample {
		t.Errorf("document header = %+v", reread)
	}
	if reread.Servers[0].Provenance.CanonicalSHA256 != row.Provenance.CanonicalSHA256 {
		t.Error("round trip lost the canonical hash")
	}
	// The site reads these exact keys.
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	server := shape["servers"].([]any)[0].(map[string]any)
	for _, key := range []string{"id", "name", "maintainer", "status", "tool_count", "protocol_revision", "provenance", "counts", "modes"} {
		if _, ok := server[key]; !ok {
			t.Errorf("published row is missing %q", key)
		}
	}
	counts := server["counts"].(map[string]any)
	for _, key := range []string{"openai_o200k", "claude", "gemini"} {
		if _, ok := counts[key]; !ok {
			t.Errorf("counts is missing %q", key)
		}
	}
}

// failingCounter fails the o200k_base count after failAfter successful calls.
// Call 1 is the whole-surface total; the rest are per-tool.
type failingCounter struct {
	failAfter int
	calls     int
}

func (f *failingCounter) Count(string) (int, error) {
	f.calls++
	if f.calls > f.failAfter {
		return 0, errors.New("o200k_base ranks unavailable")
	}
	return 100, nil
}

// A server that enumerates but cannot be counted is still status ok, because
// nothing about the server failed. What it must never do is publish the
// unmeasured count as a measured zero.
func TestFailedOpenAICountPublishesNoModes(t *testing.T) {
	cases := []struct {
		name      string
		failAfter int
	}{
		{"total_count_fails", 0},
		{"per_tool_count_fails_partway", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.StepTimeout = 20 * time.Second
			cfg.ServerTimeout = 40 * time.Second
			cfg.ScratchDir = t.TempDir()

			r := NewRunner(cfg)
			r.openai = &failingCounter{failAfter: tc.failAfter}
			doc := r.Run(context.Background(), []corpus.Server{fakeEntry()})
			row := doc.Servers[0]

			if row.Status != report.StatusOK {
				t.Errorf("status = %s (%s), want ok: enumeration succeeded", row.Status, row.Error)
			}
			if row.ToolCount != 2 {
				t.Errorf("tool_count = %d, want the enumerated count", row.ToolCount)
			}
			if row.Modes != nil {
				t.Errorf("modes published without an o200k count: %+v", *row.Modes)
			}
			if row.Counts.OpenAI.Available {
				t.Error("openai cell reported available after a count error")
			}
			if row.Counts.OpenAI.Error == "" {
				t.Error("openai cell dropped the count error")
			}
			if row.Counts.OpenAI.TotalSchemaTokens != 0 || len(row.Counts.OpenAI.PerTool) != 0 {
				t.Errorf("openai cell published figures anyway: %+v", row.Counts.OpenAI)
			}

			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"measured"`) {
				t.Errorf("an uncounted row published a measured figure: %s", raw)
			}
			var shape map[string]any
			if err := json.Unmarshal(raw, &shape); err != nil {
				t.Fatal(err)
			}
			server := shape["servers"].([]any)[0].(map[string]any)
			m, ok := server["modes"]
			if !ok {
				t.Fatal("published row dropped the modes key entirely")
			}
			if m != nil {
				t.Errorf("modes = %v, want null", m)
			}
		})
	}
}

// An operator abort is not a measurement. Methodology 7's classes are all
// server-attributable, so an interrupted sweep publishes the completed rows and
// a run-level marker, never a bogus timeout row for a server it never reached.
func TestOperatorAbortPublishesNoRowForUnsweptServers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StepTimeout = 20 * time.Second
	cfg.ServerTimeout = 40 * time.Second
	cfg.ScratchDir = t.TempDir()

	first := fakeEntry()
	second := fakeEntry()
	second.ID = "fake-two"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel once the first server has produced its row, standing in for a
	// Ctrl-C partway through a sweep.
	cfg.Logf = func(string, ...any) {}
	r := NewRunner(cfg)
	doc := r.Run(ctx, []corpus.Server{first})
	if len(doc.Servers) != 1 || doc.Run.Aborted {
		t.Fatalf("uninterrupted run = %d rows, aborted=%v", len(doc.Servers), doc.Run.Aborted)
	}

	cancel()
	aborted := r.Run(ctx, []corpus.Server{first, second})
	if !aborted.Run.Aborted {
		t.Error("an interrupted run was not marked aborted")
	}
	for _, row := range aborted.Servers {
		if row.Status == report.StatusTimeout {
			t.Errorf("%s published a timeout row for an operator abort", row.ID)
		}
		if row.Status == statusAborted {
			t.Errorf("%s published an abort row instead of being dropped", row.ID)
		}
	}
}

func TestSweepFailureRowsDoNotStopTheSweep(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StepTimeout = 5 * time.Second
	cfg.ServerTimeout = 10 * time.Second
	cfg.ScratchDir = t.TempDir()

	unresolved := corpus.Server{
		ID: "unresolved", Name: "No spec", MaintainerType: "community",
		Transport: []string{"stdio"},
		Auth:      corpus.Auth{Required: ptrBool(false)},
		Package:   map[string]corpus.Pkg{"stdio": {Type: "binary"}},
	}
	noCredential := corpus.Server{
		ID: "needs-auth", Name: "Auth server", MaintainerType: "official",
		Transport: []string{"remote"},
		Endpoint:  ptrString("https://example.invalid/mcp"),
		Auth:      corpus.Auth{Required: ptrBool(true), TokenEnv: ptrString("LOADLINE_TEST_ABSENT_TOKEN")},
	}
	noConvention := corpus.Server{
		ID: "no-convention", Name: "Undeclared credential", MaintainerType: "community",
		Transport: []string{"remote"},
		Endpoint:  ptrString("https://example.invalid/mcp"),
		Auth:      corpus.Auth{Required: ptrBool(true)},
	}
	missingBinary := corpus.Server{
		ID: "missing-binary", Name: "Absent executable", MaintainerType: "community",
		Transport: []string{"stdio"},
		Auth:      corpus.Auth{Required: ptrBool(false)},
		Package:   map[string]corpus.Pkg{"stdio": {Type: "binary", Command: "loadline-no-such-binary-xyz"}},
	}

	doc := NewRunner(cfg).Run(context.Background(), []corpus.Server{
		unresolved, noCredential, noConvention, missingBinary, fakeEntry(),
	})
	if len(doc.Servers) != 5 {
		t.Fatalf("rows = %d, want 5; a failure stopped the sweep", len(doc.Servers))
	}
	want := map[string]string{
		"unresolved":     report.StatusUnreachable,
		"needs-auth":     report.StatusAuth,
		"no-convention":  report.StatusAuth,
		"missing-binary": report.StatusUnreachable,
		"fake":           report.StatusOK,
	}
	for _, row := range doc.Servers {
		if got := want[row.ID]; row.Status != got {
			t.Errorf("%s status = %s (%s), want %s", row.ID, row.Status, row.Error, got)
		}
		if row.Status != report.StatusOK && row.Error == "" {
			t.Errorf("%s failure row carries no raw error", row.ID)
		}
		if row.MethodologyVer == "" || row.HarnessVer == "" {
			t.Errorf("%s row is missing a stamp field", row.ID)
		}
		if row.Status == report.StatusOK {
			continue
		}
		if row.Modes != nil {
			t.Errorf("%s failure row published a mode block: %+v", row.ID, *row.Modes)
		}
		if row.Counts.OpenAI.Available || row.Counts.Claude.Available || row.Counts.Gemini.Available {
			t.Errorf("%s failure row published an available cell", row.ID)
		}
	}
}

func TestAcquisitionRecordKeepsTheScratchToken(t *testing.T) {
	file, err := corpus.Load(filepath.Join("..", "..", "servers.yaml"))
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	var fsEntry corpus.Server
	for _, s := range file.Servers {
		if s.ID == "filesystem" {
			fsEntry = s
		}
	}
	l, err := resolveLaunch(fsEntry, "/tmp/loadline-scratch-12345")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	acq := acquisitionOf(l, nil)
	if fmt.Sprint(acq.Args) != fmt.Sprint([]string{"-y", "@modelcontextprotocol/server-filesystem", scratchToken}) {
		t.Errorf("recorded args = %v, want the reproducible form", acq.Args)
	}
	if l.args[2] != "/tmp/loadline-scratch-12345" {
		t.Errorf("executed args were not substituted: %v", l.args)
	}
}

func TestPypiConstraintIsRecordedInThePin(t *testing.T) {
	s := corpus.Server{
		ID: "constrained", Transport: []string{"stdio"},
		Auth: corpus.Auth{Required: ptrBool(false)},
		Package: map[string]corpus.Pkg{"stdio": {
			Type: "pypi", Name: "mcp-server-fetch", Version: "2026.7.10", With: []string{"mcp<2"},
		}},
	}
	l, err := resolveLaunch(s, "/scratch")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := []string{"--with", "mcp<2", "mcp-server-fetch==2026.7.10"}
	if fmt.Sprint(l.args) != fmt.Sprint(want) {
		t.Errorf("args = %v, want %v", l.args, want)
	}
	if !strings.Contains(l.source, "--with mcp<2") {
		t.Errorf("source = %q, the constraint must be visible in the pin", l.source)
	}
	if !l.pinned {
		t.Error("a versioned pypi spec must report as pinned")
	}
}

func TestClassify(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expireCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expireCancel()

	cases := []struct {
		name string
		ctx  context.Context
		err  error
		want string
	}{
		{"auth_http", context.Background(), &mcpwire.AuthError{Status: http.StatusUnauthorized, Body: "no"}, report.StatusAuth},
		{"protocol", context.Background(), &mcpwire.ProtocolError{Detail: "malformed"}, report.StatusProtocolError},
		// A genuine exceeded budget is a publishable methodology 7 class; an
		// operator cancel is not, and the two must not collapse together.
		{"deadline_err", context.Background(), context.DeadlineExceeded, report.StatusTimeout},
		{"deadline_ctx", expired, errors.New("read: interrupted"), report.StatusTimeout},
		{"cancelled_err", context.Background(), context.Canceled, statusAborted},
		{"cancelled_ctx", cancelled, errors.New("read: interrupted"), statusAborted},
		{"missing_exe", context.Background(), errors.New(`exec: "npx": executable file not found in $PATH`), report.StatusUnreachable},
		{"refused", context.Background(), errors.New("dial tcp 127.0.0.1:1: connect: connection refused"), report.StatusUnreachable},
		{"dns", context.Background(), errors.New(`dial tcp: lookup example.invalid: no such host`), report.StatusUnreachable},
		{"early_exit", context.Background(), errors.New("server closed stdout before responding: boom"), report.StatusUnreachable},
		{"rpc_auth", context.Background(), &mcpwire.RPCError{Code: -32000, Message: "Unauthorized"}, report.StatusAuth},
		{"rpc_other", context.Background(), &mcpwire.RPCError{Code: -32000, Message: "boom"}, report.StatusProtocolError},
		{"unknown", context.Background(), errors.New("something else"), report.StatusProtocolError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := classify(tc.ctx, tc.err)
			if got != tc.want {
				t.Errorf("classify = %s, want %s", got, tc.want)
			}
			if detail == "" {
				t.Error("classify dropped the raw error")
			}
		})
	}
	if status, _ := classify(context.Background(), nil); status != report.StatusOK {
		t.Errorf("nil error classified as %s", status)
	}
}

func TestResolveLaunchFromCorpus(t *testing.T) {
	file, err := corpus.Load(filepath.Join("..", "..", "servers.yaml"))
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	byID := map[string]corpus.Server{}
	for _, s := range file.Servers {
		byID[s.ID] = s
	}

	t.Run("filesystem_gets_a_root_argument", func(t *testing.T) {
		l, err := resolveLaunch(byID["filesystem"], "/scratch")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if l.command != "npx" {
			t.Errorf("command = %q", l.command)
		}
		want := []string{"-y", "@modelcontextprotocol/server-filesystem", "/scratch"}
		if fmt.Sprint(l.args) != fmt.Sprint(want) {
			t.Errorf("args = %v, want %v", l.args, want)
		}
		if l.pinned {
			t.Error("an unversioned npm spec must not report as pinned")
		}
	})

	t.Run("fetch_resolves_from_the_corpus_package_block", func(t *testing.T) {
		l, err := resolveLaunch(byID["fetch"], "/scratch")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if l.command != "uvx" || fmt.Sprint(l.args) != fmt.Sprint([]string{"mcp-server-fetch"}) {
			t.Errorf("launch = %+v", l)
		}
		if l.source != "pypi:mcp-server-fetch" {
			t.Errorf("source = %q, the pin must come from the corpus", l.source)
		}
	})

	// Every launch plan lives in servers.yaml. A per-server table in Go would
	// put half the acquisition record outside the file the run record cites.
	t.Run("no_server_is_special_cased_in_go", func(t *testing.T) {
		bare := corpus.Server{ID: "filesystem", Transport: []string{"stdio"}}
		if _, err := resolveLaunch(bare, "/scratch"); err == nil {
			t.Error("filesystem resolved with an empty corpus package block")
		}
		bare.ID = "fetch"
		if _, err := resolveLaunch(bare, "/scratch"); err == nil {
			t.Error("fetch resolved with an empty corpus package block")
		}
	})

	t.Run("todo_stub_is_unresolved", func(t *testing.T) {
		if _, err := resolveLaunch(byID["kubernetes"], "/scratch"); err == nil {
			t.Error("a docker spec with no image resolved anyway")
		}
	})

	t.Run("remote_only_uses_the_endpoint", func(t *testing.T) {
		l, err := resolveLaunch(byID["linear"], "/scratch")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if l.transport != "remote" || l.endpoint != "https://mcp.linear.app/mcp" {
			t.Errorf("launch = %+v", l)
		}
	})

	t.Run("no_endpoint_and_no_package_is_unresolved", func(t *testing.T) {
		if _, err := resolveLaunch(byID["cloudflare"], "/scratch"); err == nil {
			t.Error("cloudflare resolved despite having no single endpoint")
		}
	})

	t.Run("stdio_beats_remote_when_both_exist", func(t *testing.T) {
		l, err := resolveLaunch(byID["notion"], "/scratch")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if l.transport != "stdio" {
			t.Errorf("transport = %q, want stdio for the stronger pin", l.transport)
		}
	})
}

func TestCredential(t *testing.T) {
	t.Setenv("LOADLINE_TEST_TOKEN", "secret-value")

	s := corpus.Server{Auth: corpus.Auth{Required: ptrBool(true), TokenEnv: ptrString("LOADLINE_TEST_TOKEN")}}
	name, value, err := credential(s)
	if err != nil || name != "LOADLINE_TEST_TOKEN" || value != "secret-value" {
		t.Fatalf("credential = (%q, %q, %v)", name, value, err)
	}

	s.Auth.TokenEnv = ptrString("LOADLINE_TEST_ABSENT")
	if _, _, err := credential(s); err == nil {
		t.Error("an unset credential must fail the row")
	}

	s.Auth = corpus.Auth{Required: ptrBool(false)}
	if _, _, err := credential(s); err != nil {
		t.Errorf("an auth-free server must not need a credential: %v", err)
	}

	// A null auth.required in the corpus means unresearched, which is treated
	// as required so no partial surface is published silently.
	s.Auth = corpus.Auth{}
	if _, _, err := credential(s); err == nil {
		t.Error("an unresearched auth block must not pass as auth-free")
	}
}

func TestInheritedEnvCarriesNoSecrets(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "must-not-leak")
	for _, kv := range inheritedEnv() {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") {
			t.Fatal("harness credentials leaked into a server subprocess")
		}
	}
}
