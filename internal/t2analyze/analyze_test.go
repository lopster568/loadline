package t2analyze

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden report")

// goldenSource is the Source stamped into the golden report. It is a fixed
// string rather than the absolute path of whoever ran the test, so the golden
// file stays reproducible on any machine.
const goldenSource = "testdata/sample.jsonl"

func analyzeSample(t *testing.T) *Report {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rep, err := Analyze(f, Options{Source: goldenSource})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return rep
}

// TestGolden pins the whole report shape. Every other test in this file
// asserts one property; this one catches a field that silently changed
// meaning, which is the failure mode that matters when the analyzer version
// is a comparability stamp (section 5).
func TestGolden(t *testing.T) {
	rep := analyzeSample(t)
	got, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	path := filepath.Join("testdata", "sample.golden.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run go test -run TestGolden -update to create it): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("report differs from golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestHeaderStamps(t *testing.T) {
	rep := analyzeSample(t)
	if rep.AnalyzerVersion != Version {
		t.Errorf("analyzer version = %q, want %q", rep.AnalyzerVersion, Version)
	}
	if rep.InterposerVersion != "0.1.0" {
		t.Errorf("interposer version = %q, want 0.1.0", rep.InterposerVersion)
	}
	if rep.Sessions != 1 {
		t.Errorf("sessions = %d, want 1", rep.Sessions)
	}
	if rep.Tokenizer != "o200k_base" {
		t.Errorf("tokenizer = %q, want o200k_base", rep.Tokenizer)
	}
	if len(rep.ServerCmd) == 0 || rep.ServerCmd[0] != "npx" {
		t.Errorf("server cmd = %v, want the header argv", rep.ServerCmd)
	}
	if len(rep.InterposerVersions) != 0 {
		t.Errorf("single-version log should not list versions, got %v", rep.InterposerVersions)
	}
}

// TestCorrelation covers the point of the whole package: a request and its
// response are joined by id across intervening frames, including when either
// side arrives inside a batch, and including a string id that must not be
// confused with the numeric id of the same value.
func TestCorrelation(t *testing.T) {
	rep := analyzeSample(t)
	if got, want := len(rep.ToolCalls), 5; got != want {
		t.Fatalf("tool calls = %d, want %d", got, want)
	}
	wantTools := []string{"list_directory", "read_file", "read_file", "read_file", "write_file"}
	for i, want := range wantTools {
		if rep.ToolCalls[i].Tool != want {
			t.Errorf("call %d tool = %q, want %q", i+1, rep.ToolCalls[i].Tool, want)
		}
		if rep.ToolCalls[i].Seq != i+1 {
			t.Errorf("call %d seq = %d, want %d", i+1, rep.ToolCalls[i].Seq, i+1)
		}
	}

	// The batched call and the batched response correlate to each other.
	batched := rep.ToolCalls[0]
	if batched.ID != "3" || batched.Response == nil {
		t.Fatalf("batched call 3 did not correlate: %+v", batched)
	}
	if batched.Response.ResultBytes != 312 {
		t.Errorf("call 3 result bytes = %d, want 312", batched.Response.ResultBytes)
	}
	if batched.Response.LatencyMS == nil || *batched.Response.LatencyMS != 140 {
		t.Errorf("call 3 latency = %v, want 140ms", batched.Response.LatencyMS)
	}

	// The last call is never answered, so its response stays nil rather than
	// being filled with zeroes.
	last := rep.ToolCalls[4]
	if last.ID != `"call-7"` {
		t.Errorf("string id rendered as %q, want a quoted JSON string", last.ID)
	}
	if last.Response != nil {
		t.Errorf("unanswered call should have a nil response, got %+v", last.Response)
	}
	if rep.Totals.UnmatchedRequests != 1 {
		t.Errorf("unmatched requests = %d, want 1", rep.Totals.UnmatchedRequests)
	}
	if rep.Totals.UnmatchedResponses != 1 {
		t.Errorf("unmatched responses = %d, want 1", rep.Totals.UnmatchedResponses)
	}
}

func TestBatchFrames(t *testing.T) {
	rep := analyzeSample(t)
	if rep.Totals.BatchFrames != 2 {
		t.Errorf("batch frames = %d, want 2", rep.Totals.BatchFrames)
	}
	if rep.Totals.BatchMessages != 4 {
		t.Errorf("batch messages = %d, want 4", rep.Totals.BatchMessages)
	}
	// A batch frame's size_bytes covers the whole array, so it is counted
	// once toward the direction total and never split across its messages.
	if rep.Totals.Frames != 14 {
		t.Errorf("frames = %d, want 14", rep.Totals.Frames)
	}
}

func TestUnparseable(t *testing.T) {
	rep := analyzeSample(t)
	if rep.Totals.UnparseableLines != 1 {
		t.Errorf("unparseable lines = %d, want 1", rep.Totals.UnparseableLines)
	}
	if rep.Totals.UnparseableFrames != 1 {
		t.Errorf("unparseable frames = %d, want 1", rep.Totals.UnparseableFrames)
	}
	// A garbage line is data about the run, not a reason to abandon it.
	if rep.Totals.ToolCalls == 0 {
		t.Error("analysis stopped at the unparseable line")
	}
}

func TestErrorsAndRetries(t *testing.T) {
	rep := analyzeSample(t)
	if rep.Totals.JSONRPCErrors != 1 {
		t.Errorf("jsonrpc errors = %d, want 1", rep.Totals.JSONRPCErrors)
	}
	if rep.Totals.ToolErrors != 1 {
		t.Errorf("tool errors = %d, want 1", rep.Totals.ToolErrors)
	}

	errCall := rep.ToolCalls[2]
	if errCall.Response == nil || errCall.Response.RPCError == nil {
		t.Fatalf("call 5 lost its JSON-RPC error: %+v", errCall)
	}
	if errCall.Response.RPCError.Code != -32602 {
		t.Errorf("error code = %d, want -32602", errCall.Response.RPCError.Code)
	}

	toolErrCall := rep.ToolCalls[3]
	if toolErrCall.Response == nil || !toolErrCall.Response.IsToolError {
		t.Errorf("call 6 should carry the tool-level error flag: %+v", toolErrCall)
	}

	// Call 6 repeats call 5's arguments with the params keys in a different
	// order. Section 4 counts that as a retry, so key order must not defeat
	// the comparison.
	if rep.Totals.Retries != 1 {
		t.Errorf("retries = %d, want 1", rep.Totals.Retries)
	}
	if !rep.ToolCalls[3].Retry {
		t.Error("call 6 is a repeat of call 5 and should be marked a retry")
	}
	if rep.ToolCalls[2].Retry {
		t.Error("call 5 is the first of its pair and is not a retry")
	}
}

func TestResultTokensWithheldWithoutFullResults(t *testing.T) {
	rep := analyzeSample(t)
	if rep.Totals.ToolCallResultTokens != nil {
		t.Errorf("result tokens = %v, want nil without --full-results", *rep.Totals.ToolCallResultTokens)
	}
	if rep.Totals.ToolCallResultBytes == 0 {
		t.Error("result bytes are available from result_summary and should be counted")
	}
	if !hasWarning(rep, "result tokens unavailable") {
		t.Errorf("missing the withheld-measurement warning, got %v", rep.Warnings)
	}
}

func TestResultTokensCountedWithFullResults(t *testing.T) {
	log := strings.Join([]string{
		`{"interposer_version":"0.1.0","started_at":"2026-08-18T10:00:00Z","server_cmd":["srv"],"pid":1}`,
		`{"ts":"2026-08-18T10:00:00.100000000Z","dir":"c2s","size_bytes":90,"method":"tools/call","id":1,"params_full":{"name":"read_file","arguments":{"path":"notes.txt"}}}`,
		`{"ts":"2026-08-18T10:00:00.300000000Z","dir":"s2c","size_bytes":120,"id":1,"is_response":true,"result_summary":{"bytes":70,"content_blocks":1,"text_len":20},"result_full":{"content":[{"type":"text","text":"seed value: baseline"}]}}`,
	}, "\n")

	rep, err := Analyze(strings.NewReader(log), Options{Source: "inline"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.ToolCallResultTokens == nil {
		t.Fatalf("result tokens should be measured when result_full is present: %v", rep.Warnings)
	}
	if *rep.Totals.ToolCallResultTokens <= 0 {
		t.Errorf("result tokens = %d, want a positive count", *rep.Totals.ToolCallResultTokens)
	}
	rt := rep.ToolCalls[0].Response.ResultTokens
	if rt == nil || *rt != *rep.Totals.ToolCallResultTokens {
		t.Errorf("per-call result tokens = %v, want the single-call total", rt)
	}
	if hasWarning(rep, "result tokens unavailable") {
		t.Errorf("unexpected withheld warning: %v", rep.Warnings)
	}
}

func TestArgumentSizes(t *testing.T) {
	rep := analyzeSample(t)
	var byteSum, tokenSum int
	for _, c := range rep.ToolCalls {
		if c.ArgBytes <= 0 {
			t.Errorf("call %d has no argument bytes", c.Seq)
		}
		if c.ArgTokens <= 0 {
			t.Errorf("call %d has no argument tokens", c.Seq)
		}
		byteSum += c.ArgBytes
		tokenSum += c.ArgTokens
	}
	if rep.Totals.ToolCallArgBytes != byteSum {
		t.Errorf("arg byte total = %d, want %d", rep.Totals.ToolCallArgBytes, byteSum)
	}
	if rep.Totals.ToolCallArgTokens != tokenSum {
		t.Errorf("arg token total = %d, want %d", rep.Totals.ToolCallArgTokens, tokenSum)
	}
}

func TestMethodBreakdown(t *testing.T) {
	rep := analyzeSample(t)
	byName := map[string]MethodStat{}
	var prev string
	for _, m := range rep.Methods {
		if prev != "" && m.Method < prev {
			t.Errorf("methods are not sorted: %q after %q", m.Method, prev)
		}
		prev = m.Method
		byName[m.Method] = m
	}

	call, ok := byName["tools/call"]
	if !ok {
		t.Fatalf("no tools/call row in %v", byName)
	}
	if call.Requests != 5 {
		t.Errorf("tools/call requests = %d, want 5", call.Requests)
	}
	if call.Responses != 4 {
		t.Errorf("tools/call responses = %d, want 4", call.Responses)
	}
	if call.Errors != 1 || call.ToolErrors != 1 {
		t.Errorf("tools/call errors = %d/%d, want 1 jsonrpc and 1 tool", call.Errors, call.ToolErrors)
	}

	if n := byName["notifications/initialized"].Notifications; n != 1 {
		t.Errorf("notification count = %d, want 1", n)
	}
	if byName["initialize"].Requests != 1 || byName["tools/list"].Requests != 1 {
		t.Errorf("handshake methods missing from the breakdown: %v", byName)
	}
	// The response with no logged request lands in its own bucket rather
	// than being attributed to a method it was never observed to belong to.
	if byName["(unmatched)"].Responses != 1 {
		t.Errorf("unmatched response row = %+v", byName["(unmatched)"])
	}
}

func TestWallTime(t *testing.T) {
	rep := analyzeSample(t)
	if rep.Wall.FirstFrame != "2026-08-18T10:00:00.100000000Z" {
		t.Errorf("first frame = %q", rep.Wall.FirstFrame)
	}
	if rep.Wall.LastFrame != "2026-08-18T10:00:01.000000000Z" {
		t.Errorf("last frame = %q", rep.Wall.LastFrame)
	}
	if rep.Wall.DurationMS == nil || *rep.Wall.DurationMS != 900 {
		t.Errorf("duration = %v, want 900ms", rep.Wall.DurationMS)
	}
}

func TestNoHeaderIsFlagged(t *testing.T) {
	log := `{"ts":"2026-08-18T10:00:00.100000000Z","dir":"c2s","size_bytes":40,"method":"tools/list","id":1,"params_full":{}}`
	rep, err := Analyze(strings.NewReader(log), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.InterposerVersion != "" {
		t.Errorf("version = %q, want empty", rep.InterposerVersion)
	}
	if !hasWarning(rep, "no header line") {
		t.Errorf("a log with no version stamp must be flagged, got %v", rep.Warnings)
	}
}

func TestMixedVersionsAreFlagged(t *testing.T) {
	log := strings.Join([]string{
		`{"interposer_version":"0.1.0","started_at":"2026-08-18T10:00:00Z","server_cmd":["srv"],"pid":1}`,
		`{"ts":"2026-08-18T10:00:00.100000000Z","dir":"c2s","size_bytes":40,"method":"tools/list","id":1,"params_full":{}}`,
		`{"interposer_version":"0.2.0","started_at":"2026-08-18T10:05:00Z","server_cmd":["srv"],"pid":2}`,
		`{"ts":"2026-08-18T10:05:00.100000000Z","dir":"c2s","size_bytes":40,"method":"tools/list","id":1,"params_full":{}}`,
	}, "\n")

	rep, err := Analyze(strings.NewReader(log), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Sessions != 2 {
		t.Errorf("sessions = %d, want 2", rep.Sessions)
	}
	if len(rep.InterposerVersions) != 2 {
		t.Errorf("versions = %v, want both listed", rep.InterposerVersions)
	}
	if !hasWarning(rep, "not comparable") {
		t.Errorf("mixed versions must be flagged as non-comparable, got %v", rep.Warnings)
	}
	// The second session reuses id 1 while the first is still outstanding.
	if rep.Totals.DuplicateRequestID != 1 {
		t.Errorf("duplicate request ids = %d, want 1", rep.Totals.DuplicateRequestID)
	}
}

// TestCounterFailureAborts holds the line from methodology 1.7: a token count
// that could not be taken is never published as a zero, so a broken tokenizer
// fails the analysis instead of producing a report full of plausible zeroes.
func TestCounterFailureAborts(t *testing.T) {
	log := `{"ts":"2026-08-18T10:00:00.100000000Z","dir":"c2s","size_bytes":90,"method":"tools/call","id":1,"params_full":{"name":"read_file"}}`
	_, err := Analyze(strings.NewReader(log), Options{Counter: brokenCounter{}, Encoding: "broken"})
	if err == nil {
		t.Fatal("expected an error when the tokenizer fails")
	}
	if !strings.Contains(err.Error(), "count params tokens") {
		t.Errorf("error = %v, want it to name the failing step", err)
	}
}

func TestEmptyLog(t *testing.T) {
	rep, err := Analyze(strings.NewReader(""), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Totals.Frames != 0 || len(rep.ToolCalls) != 0 {
		t.Errorf("empty log produced content: %+v", rep.Totals)
	}
	if rep.Wall.DurationMS != nil {
		t.Errorf("duration = %v, want nil for a log with no frames", *rep.Wall.DurationMS)
	}
}

type brokenCounter struct{}

func (brokenCounter) Count(string) (int, error) { return 0, errors.New("tokenizer unavailable") }

func hasWarning(rep *Report, substr string) bool {
	for _, w := range rep.Warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestMetaStrippedResultTokens(t *testing.T) {
	header := `{"interposer_version":"0.1.0","started_at":"2026-08-18T10:00:00Z","server_cmd":["srv"],"pid":1}`
	req1 := `{"ts":"2026-08-18T10:00:00.100000000Z","dir":"c2s","size_bytes":90,"method":"tools/call","id":1,"params_full":{"name":"get_file_contents","arguments":{"path":"README.md"}}}`
	req2 := `{"ts":"2026-08-18T10:00:00.400000000Z","dir":"c2s","size_bytes":90,"method":"tools/call","id":2,"params_full":{"name":"read_file","arguments":{"path":"notes.txt"}}}`
	// resp1 carries the github-shaped envelope: _meta as a top-level sibling
	// of content. resp2 carries none, the Gemini shape.
	resp1Meta := `{"ts":"2026-08-18T10:00:00.300000000Z","dir":"s2c","size_bytes":400,"id":1,"is_response":true,"result_summary":{"bytes":300,"content_blocks":1,"text_len":20},"result_full":{"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"github","title":"GitHub","version":"1.9.0","icons":[{"src":"data:image/png;base64,AAAABBBBCCCC"}]}},"content":[{"type":"text","text":"seed value: baseline"}],"resultType":"text"}}`
	resp1Bare := `{"ts":"2026-08-18T10:00:00.300000000Z","dir":"s2c","size_bytes":400,"id":1,"is_response":true,"result_summary":{"bytes":300,"content_blocks":1,"text_len":20},"result_full":{"content":[{"type":"text","text":"seed value: baseline"}],"resultType":"text"}}`
	resp2 := `{"ts":"2026-08-18T10:00:00.600000000Z","dir":"s2c","size_bytes":120,"id":2,"is_response":true,"result_summary":{"bytes":70,"content_blocks":1,"text_len":20},"result_full":{"content":[{"type":"text","text":"note body"}]}}`

	withMeta, err := Analyze(strings.NewReader(strings.Join([]string{header, req1, resp1Meta, req2, resp2}, "\n")), Options{Source: "inline"})
	if err != nil {
		t.Fatal(err)
	}
	// The reference: the identical log with the _meta member deleted by hand.
	// Stripping must measure exactly what the server would have sent without
	// the envelope, so withMeta's stripped count equals reference's raw count.
	reference, err := Analyze(strings.NewReader(strings.Join([]string{header, req1, resp1Bare, req2, resp2}, "\n")), Options{Source: "inline"})
	if err != nil {
		t.Fatal(err)
	}

	raw := withMeta.Totals.ToolCallResultTokens
	stripped := withMeta.Totals.ToolCallResultTokensMetaStripped
	if raw == nil || stripped == nil {
		t.Fatalf("raw = %v stripped = %v, want both measured when every response carries result_full", raw, stripped)
	}
	if *stripped >= *raw {
		t.Errorf("stripped total = %d, want less than raw %d when a _meta block is present", *stripped, *raw)
	}
	if want := reference.Totals.ToolCallResultTokens; want == nil || *stripped != *want {
		t.Errorf("stripped total = %d, want %v, the raw count of the same log with _meta deleted", *stripped, want)
	}

	// Per-call: call 1 strips, call 2 is untouched (stripped == raw).
	c1 := withMeta.ToolCalls[0].Response
	if c1.ResultTokensMetaStripped == nil || c1.ResultTokens == nil || *c1.ResultTokensMetaStripped >= *c1.ResultTokens {
		t.Errorf("call 1 stripped = %v raw = %v, want stripped < raw", c1.ResultTokensMetaStripped, c1.ResultTokens)
	}
	c2 := withMeta.ToolCalls[1].Response
	if c2.ResultTokensMetaStripped == nil || c2.ResultTokens == nil || *c2.ResultTokensMetaStripped != *c2.ResultTokens {
		t.Errorf("call 2 stripped = %v raw = %v, want equal when no _meta member exists", c2.ResultTokensMetaStripped, c2.ResultTokens)
	}
	if *c1.ResultTokensMetaStripped+*c2.ResultTokensMetaStripped != *stripped {
		t.Errorf("per-call stripped sum %d != total %d", *c1.ResultTokensMetaStripped+*c2.ResultTokensMetaStripped, *stripped)
	}
}

func TestMetaStrippedWithheldWithoutFullResults(t *testing.T) {
	// Same nullability rule as ToolCallResultTokens: an estimate never
	// publishes as a measurement, so without result_full on every response
	// the stripped total is withheld too.
	rep := analyzeSample(t)
	if rep.Totals.ToolCallResultTokensMetaStripped != nil {
		t.Errorf("stripped tokens = %v, want nil without --full-results", *rep.Totals.ToolCallResultTokensMetaStripped)
	}
}
