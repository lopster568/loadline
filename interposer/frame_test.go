package interposer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// logFrames runs a list of raw wire frames through the logger and returns the
// decoded JSONL records.
func logFrames(t *testing.T, full bool, frames ...string) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	lg := &logger{w: &buf, full: full}
	for _, f := range frames {
		lg.frame(dirC2S, []byte(f))
	}
	return decodeJSONL(t, buf.Bytes())
}

func decodeJSONL(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func TestLogRequestKeepsFullParams(t *testing.T) {
	frame := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"search_issues","arguments":{"query":"is:open label:bug","limit":50}}}` + "\n"
	rec := logFrames(t, false, frame)[0]

	if rec["dir"] != "c2s" {
		t.Errorf("dir = %v, want c2s", rec["dir"])
	}
	if got, want := int(rec["size_bytes"].(float64)), len(frame); got != want {
		t.Errorf("size_bytes = %d, want %d (frame including newline)", got, want)
	}
	if rec["method"] != "tools/call" {
		t.Errorf("method = %v, want tools/call", rec["method"])
	}
	if rec["id"].(float64) != 7 {
		t.Errorf("id = %v, want 7", rec["id"])
	}
	if _, ok := rec["is_response"]; ok {
		t.Error("request must not be flagged is_response")
	}
	if rec["ts"] == "" {
		t.Error("ts is empty")
	}

	// The whole arguments object must survive; this is the gap the
	// interposer exists to close.
	params := rec["params_full"].(map[string]any)
	args := params["arguments"].(map[string]any)
	if args["query"] != "is:open label:bug" || args["limit"].(float64) != 50 {
		t.Errorf("params_full lost arguments: %v", params)
	}
}

func TestLogNotificationHasNoID(t *testing.T) {
	rec := logFrames(t, false, `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`+"\n")[0]
	if rec["method"] != "notifications/initialized" {
		t.Errorf("method = %v", rec["method"])
	}
	if _, ok := rec["id"]; ok {
		t.Errorf("notification must not carry an id: %v", rec["id"])
	}
	if _, ok := rec["is_response"]; ok {
		t.Error("notification must not be flagged is_response")
	}
}

func TestLogToolsCallResultSummary(t *testing.T) {
	result := `{"content":[{"type":"text","text":"aaaa"},{"type":"text","text":"bbbbbb"},{"type":"image","data":"zz"}],"isError":false}`
	rec := logFrames(t, false, `{"jsonrpc":"2.0","id":7,"result":`+result+`}`+"\n")[0]

	if rec["is_response"] != true {
		t.Error("response not flagged is_response")
	}
	if _, ok := rec["result_full"]; ok {
		t.Error("result_full must be absent without --full-results")
	}
	sum := rec["result_summary"].(map[string]any)
	if got, want := int(sum["bytes"].(float64)), len(result); got != want {
		t.Errorf("result bytes = %d, want %d", got, want)
	}
	if sum["content_blocks"].(float64) != 3 {
		t.Errorf("content_blocks = %v, want 3", sum["content_blocks"])
	}
	if sum["text_len"].(float64) != 10 {
		t.Errorf("text_len = %v, want 10", sum["text_len"])
	}
	if sum["is_error"] != false {
		t.Errorf("is_error = %v, want false", sum["is_error"])
	}
}

func TestLogNonToolResultOmitsContentFields(t *testing.T) {
	rec := logFrames(t, false, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"a"}]}}`+"\n")[0]
	sum := rec["result_summary"].(map[string]any)
	if _, ok := sum["content_blocks"]; ok {
		t.Error("content_blocks must be absent for a non tools/call result")
	}
	if sum["bytes"].(float64) == 0 {
		t.Error("bytes must still be recorded")
	}
}

func TestLogFullResults(t *testing.T) {
	rec := logFrames(t, true, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"hi"}]}}`+"\n")[0]
	full := rec["result_full"].(map[string]any)
	blocks := full["content"].([]any)
	if blocks[0].(map[string]any)["text"] != "hi" {
		t.Errorf("result_full did not preserve the payload: %v", full)
	}
	if rec["result_summary"] == nil {
		t.Error("result_summary must still be present with --full-results")
	}
}

func TestLogErrorResponse(t *testing.T) {
	rec := logFrames(t, false, `{"jsonrpc":"2.0","id":9,"error":{"code":-32602,"message":"invalid params","data":{"field":"name"}}}`+"\n")[0]
	if rec["is_response"] != true {
		t.Error("error response not flagged is_response")
	}
	e := rec["error"].(map[string]any)
	if e["code"].(float64) != -32602 || e["message"] != "invalid params" {
		t.Errorf("error = %v", e)
	}
	if _, ok := rec["error_full"]; ok {
		t.Error("error_full must be absent without --full-results")
	}

	rec = logFrames(t, true, `{"jsonrpc":"2.0","id":9,"error":{"code":-32602,"message":"invalid params","data":{"field":"name"}}}`+"\n")[0]
	if rec["error_full"].(map[string]any)["data"] == nil {
		t.Error("error_full must carry error data with --full-results")
	}
}

func TestLogBatch(t *testing.T) {
	frame := `[{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}},{"jsonrpc":"2.0","method":"notifications/cancelled"},{"jsonrpc":"2.0","id":2,"result":{"ok":true}}]` + "\n"
	rec := logFrames(t, false, frame)[0]

	if rec["batch"] != true {
		t.Fatalf("batch flag missing: %v", rec)
	}
	if rec["batch_len"].(float64) != 3 {
		t.Errorf("batch_len = %v, want 3", rec["batch_len"])
	}
	items := rec["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if items[0].(map[string]any)["method"] != "tools/list" {
		t.Errorf("item 0 = %v", items[0])
	}
	if _, ok := items[1].(map[string]any)["id"]; ok {
		t.Errorf("item 1 is a notification and must have no id: %v", items[1])
	}
	if items[2].(map[string]any)["is_response"] != true {
		t.Errorf("item 2 = %v", items[2])
	}
	// Batch frames must not also promote message fields to the top level.
	if _, ok := rec["method"]; ok {
		t.Errorf("batch record leaked a top-level method: %v", rec)
	}
}

func TestLogUnparseable(t *testing.T) {
	cases := []string{
		"this is not json\n",
		"{broken\n",
		"[1,2,\n",
		"\n",
	}
	for _, frame := range cases {
		rec := logFrames(t, false, frame)[0]
		if rec["unparseable"] != true {
			t.Errorf("frame %q: unparseable = %v, want true", frame, rec["unparseable"])
		}
		if int(rec["size_bytes"].(float64)) != len(frame) {
			t.Errorf("frame %q: size_bytes = %v, want %d", frame, rec["size_bytes"], len(frame))
		}
	}
}

func TestLogBatchWithUnparseableItem(t *testing.T) {
	rec := logFrames(t, false, `[{"jsonrpc":"2.0","id":1,"method":"ping"},"not-a-message"]`+"\n")[0]
	items := rec["items"].([]any)
	if items[1].(map[string]any)["unparseable"] != true {
		t.Errorf("item 1 should be unparseable: %v", items[1])
	}
	if items[0].(map[string]any)["method"] != "ping" {
		t.Errorf("item 0 should still parse: %v", items[0])
	}
}

func TestLogStringIDPreserved(t *testing.T) {
	rec := logFrames(t, false, `{"jsonrpc":"2.0","id":"req-abc","method":"ping"}`+"\n")[0]
	if rec["id"] != "req-abc" {
		t.Errorf("id = %#v, want the string req-abc", rec["id"])
	}
}

func TestLogOneObjectPerFrame(t *testing.T) {
	recs := logFrames(t, false,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n",
		"garbage\n",
		`{"jsonrpc":"2.0","id":1,"result":{}}`+"\n",
	)
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3", len(recs))
	}
}
