package interposer

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The tests spawn this test binary as the fake MCP server. LOADLINE_TEST_MODE
// selects the behaviour; without it the binary runs the tests as usual.
const modeEnv = "LOADLINE_TEST_MODE"

func TestMain(m *testing.M) {
	switch os.Getenv(modeEnv) {
	case "":
		os.Exit(m.Run())
	case "echo":
		// Byte-exact echo, then exit when the client closes stdin.
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "exit7":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(7)
	case "stderr":
		fmt.Fprintln(os.Stderr, "server-stderr-marker")
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "emit":
		// Emit one frame of the requested size before draining stdin.
		n, _ := strconv.Atoi(os.Getenv("LOADLINE_TEST_SIZE"))
		_, _ = os.Stdout.Write(append(bytes.Repeat([]byte("x"), n), '\n'))
		_, _ = io.Copy(io.Discard, os.Stdin)
	default:
		fmt.Fprintln(os.Stderr, "unknown test mode")
		os.Exit(3)
	}
	os.Exit(0)
}

type session struct {
	code    int
	err     error
	out     string
	stderr  string
	records []map[string]any
	logPath string
}

// runSession proxies clientIn through a fake server running in the named mode.
func runSession(t *testing.T, mode string, clientIn string, opts ...func(*Options)) session {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "frames.jsonl")
	s := runSessionAt(t, logPath, mode, clientIn, opts...)
	return s
}

func runSessionAt(t *testing.T, logPath, mode, clientIn string, opts ...func(*Options)) session {
	t.Helper()
	var out, errOut bytes.Buffer
	o := Options{
		LogPath:   logPath,
		ServerCmd: []string{os.Args[0]},
		ClientIn:  strings.NewReader(clientIn),
		ClientOut: &out,
		ServerErr: &errOut,
	}
	for _, fn := range opts {
		fn(&o)
	}
	t.Setenv(modeEnv, mode)
	code, err := Run(o)

	logBytes, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("reading log: %v", readErr)
	}
	return session{
		code:    code,
		err:     err,
		out:     out.String(),
		stderr:  errOut.String(),
		records: decodeJSONL(t, logBytes),
		logPath: logPath,
	}
}

func framesIn(recs []map[string]any, dir string) []map[string]any {
	var out []map[string]any
	for _, r := range recs {
		if r["dir"] == dir {
			out = append(out, r)
		}
	}
	return out
}

func TestPassthroughIsByteExact(t *testing.T) {
	// Mixed framing on purpose: CRLF, an empty line, multi-byte UTF-8, and a
	// final chunk with no trailing newline.
	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\r\n" +
		"\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"text":"héllo ✓"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"ping"}` // no trailing newline

	s := runSession(t, "echo", in)
	if s.err != nil {
		t.Fatalf("Run: %v", s.err)
	}
	if s.out != in {
		t.Errorf("relay is not byte-exact\n got: %q\nwant: %q", s.out, in)
	}

	// Five frames each way: CRLF does not start a new frame, the blank line
	// does count as one, and the unterminated tail is its own frame.
	if got := len(framesIn(s.records, "c2s")); got != 5 {
		t.Errorf("c2s frames = %d, want 5", got)
	}
	if got := len(framesIn(s.records, "s2c")); got != 5 {
		t.Errorf("s2c frames = %d, want 5", got)
	}

	// The CRLF frame's size must include the carriage return.
	crlf := framesIn(s.records, "c2s")[1]
	if int(crlf["size_bytes"].(float64)) != len(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)+2 {
		t.Errorf("CRLF frame size_bytes = %v", crlf["size_bytes"])
	}
	if crlf["method"] != "notifications/initialized" {
		t.Errorf("CRLF frame did not parse: %v", crlf)
	}
	// The unterminated tail frame is still parsed and logged.
	tail := framesIn(s.records, "c2s")[4]
	if tail["method"] != "ping" {
		t.Errorf("unterminated tail frame = %v", tail)
	}
}

func TestLargeFrameRoundTrip(t *testing.T) {
	const size = 3 << 20 // 3 MiB, well past the 64 KiB read buffer
	payload := strings.Repeat("y", size)
	in := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"` + payload + `"}]}}` + "\n"

	s := runSession(t, "echo", in)
	if s.err != nil {
		t.Fatalf("Run: %v", s.err)
	}
	if s.out != in {
		t.Errorf("large frame not relayed byte-exact: got %d bytes, want %d", len(s.out), len(in))
	}

	rec := framesIn(s.records, "s2c")[0]
	if int(rec["size_bytes"].(float64)) != len(in) {
		t.Errorf("size_bytes = %v, want %d", rec["size_bytes"], len(in))
	}
	sum := rec["result_summary"].(map[string]any)
	if int(sum["text_len"].(float64)) != size {
		t.Errorf("text_len = %v, want %d", sum["text_len"], size)
	}
	if sum["content_blocks"].(float64) != 1 {
		t.Errorf("content_blocks = %v, want 1", sum["content_blocks"])
	}
	// The summary must not carry the payload.
	if _, ok := rec["result_full"]; ok {
		t.Error("result_full present without --full-results")
	}
}

func TestLargeFrameArrivingInChunks(t *testing.T) {
	// The server writes a 1 MiB frame in one call; the pipe delivers it to
	// the relay in partial reads, which must reassemble into one frame.
	const size = 1 << 20
	t.Setenv("LOADLINE_TEST_SIZE", strconv.Itoa(size))
	s := runSession(t, "emit", "")
	if s.err != nil {
		t.Fatalf("Run: %v", s.err)
	}
	if len(s.out) != size+1 {
		t.Fatalf("relayed %d bytes, want %d", len(s.out), size+1)
	}
	s2c := framesIn(s.records, "s2c")
	if len(s2c) != 1 {
		t.Fatalf("s2c frames = %d, want 1 reassembled frame", len(s2c))
	}
	if int(s2c[0]["size_bytes"].(float64)) != size+1 {
		t.Errorf("size_bytes = %v, want %d", s2c[0]["size_bytes"], size+1)
	}
	if s2c[0]["unparseable"] != true {
		t.Errorf("a frame of x's is not JSON and must be flagged: %v", s2c[0])
	}
}

func TestGarbageIsRelayedAnyway(t *testing.T) {
	in := "not json at all\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	s := runSession(t, "echo", in)
	if s.out != in {
		t.Errorf("garbage was not relayed verbatim: %q", s.out)
	}
	c2s := framesIn(s.records, "c2s")
	if c2s[0]["unparseable"] != true {
		t.Errorf("garbage frame = %v", c2s[0])
	}
	if c2s[1]["method"] != "ping" {
		t.Errorf("frame after garbage = %v", c2s[1])
	}
}

func TestExitCodePropagation(t *testing.T) {
	if s := runSession(t, "exit7", "hello\n"); s.code != 7 {
		t.Errorf("exit code = %d, want 7 (err=%v)", s.code, s.err)
	}
	if s := runSession(t, "echo", "hello\n"); s.code != 0 {
		t.Errorf("exit code = %d, want 0 (err=%v)", s.code, s.err)
	}
}

func TestServerStderrPassesThrough(t *testing.T) {
	s := runSession(t, "stderr", "hello\n")
	if !strings.Contains(s.stderr, "server-stderr-marker") {
		t.Errorf("server stderr not passed through: %q", s.stderr)
	}
	if strings.Contains(s.out, "server-stderr-marker") {
		t.Error("server stderr leaked into the client stdout stream")
	}
}

func TestHeaderLine(t *testing.T) {
	s := runSession(t, "echo", "")
	h := s.records[0]
	if h["interposer_version"] != Version {
		t.Errorf("interposer_version = %v, want %s", h["interposer_version"], Version)
	}
	if h["started_at"] == "" || h["started_at"] == nil {
		t.Error("started_at missing")
	}
	cmd := h["server_cmd"].([]any)
	if len(cmd) != 1 || cmd[0] != os.Args[0] {
		t.Errorf("server_cmd = %v", cmd)
	}
	if h["pid"].(float64) <= 0 {
		t.Errorf("pid = %v", h["pid"])
	}
}

func TestLogIsAppendOnly(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "frames.jsonl")
	runSessionAt(t, logPath, "echo", `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n")
	s := runSessionAt(t, logPath, "echo", `{"jsonrpc":"2.0","id":2,"method":"ping"}`+"\n")

	headers := 0
	for _, r := range s.records {
		if r["interposer_version"] != nil {
			headers++
		}
	}
	if headers != 2 {
		t.Errorf("found %d header lines after two sessions, want 2 (log was truncated)", headers)
	}
	if len(s.records) != 6 {
		t.Errorf("total records = %d, want 6 (2 headers + 2 frames each way)", len(s.records))
	}
}

func TestLogFilePermissions(t *testing.T) {
	s := runSession(t, "echo", "")
	info, err := os.Stat(s.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log permissions = %o, want 600 (logs carry tool arguments)", perm)
	}
}

func TestMissingServerBinary(t *testing.T) {
	_, err := Run(Options{
		LogPath:   filepath.Join(t.TempDir(), "frames.jsonl"),
		ServerCmd: []string{"loadline-no-such-binary"},
		ClientIn:  strings.NewReader(""),
		ClientOut: io.Discard,
		ServerErr: io.Discard,
	})
	if err == nil {
		t.Fatal("expected an error for a missing server binary")
	}
}

func TestArgumentValidation(t *testing.T) {
	if _, err := Run(Options{LogPath: "x"}); err == nil {
		t.Error("expected an error with no server command")
	}
	if _, err := Run(Options{ServerCmd: []string{"true"}}); err == nil {
		t.Error("expected an error with no log path")
	}
}
