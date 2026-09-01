// Package t2analyze turns an interposer frame log into the per-trial Tier 2
// metrics defined in docs/tier2-task-suites.md section 4.
//
// It lives in the main module rather than in the interposer because the
// interposer is deliberately stdlib-only (interposer/README.md, "Non-goals for
// v0.1"): bundling a tokenizer there would cost the property that makes the
// proxy auditable in one sitting. Analysis is offline work on a finished log,
// so it has no reason to share that constraint.
package t2analyze

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/lopster568/loadline/internal/tokens"
)

// Version is the analyzer's own semver. Section 5 of the task-suite spec
// forbids comparing results across instrument versions, and the analyzer is
// part of the instrument: any change to how a metric below is derived is a
// bump here, the same discipline interposer/README.md applies to itself.
const Version = "0.2.0"

// maxLineBytes bounds one JSONL record. A frame carries complete tool-call
// arguments and, under the interposer's --full-results, complete tool output,
// so bufio's default 64 KiB limit is far too small to read one back.
const maxLineBytes = 64 << 20

// maxWarnings caps the warning list so a badly damaged log cannot produce a
// report larger than the log itself. The counters stay exact; only the
// human-readable lines are truncated.
const maxWarnings = 50

// Counter counts tokens in a string. internal/tokens.OpenAI satisfies it.
type Counter interface {
	Count(text string) (int, error)
}

// Options configures one analysis run.
type Options struct {
	// Source labels the report, normally the log path. It is supplied by the
	// caller rather than read from the reader so that the report is a pure
	// function of its inputs and can be compared against a golden file.
	Source string
	// Counter counts tokens. Nil selects the pinned local o200k_base
	// tokenizer, which is the basis methodology-v0.md 1.6 fixes for every
	// published token figure.
	Counter Counter
	// Encoding names the tokenizer in the report. Empty defaults to the
	// pinned encoding when Counter is nil.
	Encoding string
}

// Report is the per-trial analysis artifact.
type Report struct {
	AnalyzerVersion string `json:"analyzer_version"`

	// InterposerVersion comes from the log's own header line. A report
	// without it is not publishable: section 5 requires the stamp, and the
	// analyzer has no way to reconstruct it.
	InterposerVersion string `json:"interposer_version"`
	// InterposerVersions is present only when one file carries headers from
	// more than one interposer version, which makes its frames
	// non-comparable with each other and the report unpublishable as-is.
	InterposerVersions []string `json:"interposer_versions,omitempty"`

	Source    string   `json:"source,omitempty"`
	Tokenizer string   `json:"tokenizer"`
	Sessions  int      `json:"sessions"`
	StartedAt string   `json:"started_at,omitempty"`
	ServerCmd []string `json:"server_cmd,omitempty"`

	Wall      Wall         `json:"wall"`
	Totals    Totals       `json:"totals"`
	ToolCalls []ToolCall   `json:"tool_calls"`
	Methods   []MethodStat `json:"methods"`
	Warnings  []string     `json:"warnings,omitempty"`
}

// Wall is the trial's observed span on the wire. It is first-frame to
// last-frame, which is strictly narrower than the runner's own wall clock
// around the client process (section 3.2): client startup and the model's
// thinking before the first call are outside it.
type Wall struct {
	FirstFrame string `json:"first_frame,omitempty"`
	LastFrame  string `json:"last_frame,omitempty"`
	// DurationMS is nil when either endpoint is missing or unparseable,
	// rather than 0, because a zero-length trial and an unmeasured one are
	// different facts (methodology 1.7).
	DurationMS *int64 `json:"duration_ms"`
}

// Totals are the per-trial counts of section 4.
type Totals struct {
	Frames    int `json:"frames"`
	FramesC2S int `json:"frames_c2s"`
	FramesS2C int `json:"frames_s2c"`
	Bytes     int `json:"bytes"`
	BytesC2S  int `json:"bytes_c2s"`
	BytesS2C  int `json:"bytes_s2c"`

	BatchFrames   int `json:"batch_frames"`
	BatchMessages int `json:"batch_messages"`

	Requests      int `json:"requests"`
	Notifications int `json:"notifications"`
	Responses     int `json:"responses"`

	ToolCalls           int `json:"tool_calls"`
	ToolCallArgBytes    int `json:"tool_call_arg_bytes"`
	ToolCallArgTokens   int `json:"tool_call_arg_tokens"`
	ToolCallResultBytes int `json:"tool_call_result_bytes"`
	// ToolCallResultTokens is non-nil only when every matched tool-call
	// response carried result_full, which happens only when the interposer
	// ran with --full-results. Without it the log holds a byte count and a
	// text length for each result but not the text, so a token count would
	// have to be estimated, and an estimate never publishes as a measurement.
	ToolCallResultTokens *int `json:"tool_call_result_tokens"`
	// ToolCallResultTokensMetaStripped is ToolCallResultTokens with each
	// response's top-level _meta member removed before counting. Some servers
	// ride an envelope there (github's serverInfo block, icons included) that
	// one client requests and another does not, so the raw figure conflates
	// what the server sent with what the protocol wrapped around it; this
	// field isolates the payload. A response with no top-level _meta counts
	// identically in both. Same nullability rule as ToolCallResultTokens.
	ToolCallResultTokensMetaStripped *int `json:"tool_call_result_tokens_meta_stripped"`

	// JSONRPCErrors counts responses carrying an error member. ToolErrors
	// counts successful responses whose result sets isError, which is a
	// different failure and is kept separate rather than summed.
	JSONRPCErrors int `json:"jsonrpc_errors"`
	ToolErrors    int `json:"tool_errors"`
	// Retries counts tools/call requests whose method and params are
	// equivalent to the immediately preceding tools/call in the same trial,
	// per section 4's retry definition.
	Retries int `json:"retries"`

	UnmatchedRequests  int `json:"unmatched_requests"`
	UnmatchedResponses int `json:"unmatched_responses"`
	DuplicateRequestID int `json:"duplicate_request_ids"`
	UnparseableLines   int `json:"unparseable_lines"`
	UnparseableFrames  int `json:"unparseable_frames"`
	UnrecognizedLines  int `json:"unrecognized_lines"`
}

// ToolCall is one tools/call request and the response correlated to it by id.
type ToolCall struct {
	Seq       int    `json:"seq"`
	ID        string `json:"id,omitempty"`
	Tool      string `json:"tool,omitempty"`
	TS        string `json:"ts,omitempty"`
	ArgBytes  int    `json:"arg_bytes"`
	ArgTokens int    `json:"arg_tokens"`
	Retry     bool   `json:"retry,omitempty"`
	// Response is nil when no response with a matching id was ever logged,
	// which is itself data: a trial can end with a call still in flight.
	Response *CallResponse `json:"response"`
}

// CallResponse is the response side of one tool call.
type CallResponse struct {
	TS            string    `json:"ts,omitempty"`
	ResultBytes   int       `json:"result_bytes"`
	ContentBlocks *int      `json:"content_blocks,omitempty"`
	TextLen       *int      `json:"text_len,omitempty"`
	IsToolError   bool      `json:"is_tool_error,omitempty"`
	RPCError      *RPCError `json:"rpc_error,omitempty"`
	// ResultTokens is nil unless the log carries result_full for this
	// response. See Totals.ToolCallResultTokens.
	ResultTokens *int `json:"result_tokens"`
	// ResultTokensMetaStripped is ResultTokens with the top-level _meta
	// member removed before counting; equal to ResultTokens when the result
	// carries none. See Totals.ToolCallResultTokensMetaStripped.
	ResultTokensMetaStripped *int   `json:"result_tokens_meta_stripped"`
	LatencyMS                *int64 `json:"latency_ms"`
}

// RPCError is the JSON-RPC error member as the interposer logged it.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MethodStat is the per-method breakdown. Frame byte sizes are deliberately
// absent: a batch frame's size_bytes covers every message in it, so it cannot
// be attributed to one method. ParamsBytes is exact either way.
type MethodStat struct {
	Method        string `json:"method"`
	Requests      int    `json:"requests"`
	Notifications int    `json:"notifications"`
	Responses     int    `json:"responses"`
	Errors        int    `json:"errors"`
	ToolErrors    int    `json:"tool_errors"`
	ParamsBytes   int    `json:"params_bytes"`
	ParamsTokens  int    `json:"params_tokens"`
	ResultBytes   int    `json:"result_bytes"`
}

// rawLine is a header line or a frame line. The two are told apart by which
// fields are present: only a header has interposer_version, only a frame has
// dir (interposer/README.md, "Log format").
type rawLine struct {
	InterposerVersion string   `json:"interposer_version"`
	StartedAt         string   `json:"started_at"`
	ServerCmd         []string `json:"server_cmd"`

	TS        string   `json:"ts"`
	Dir       string   `json:"dir"`
	SizeBytes int      `json:"size_bytes"`
	Batch     bool     `json:"batch"`
	BatchLen  int      `json:"batch_len"`
	Items     []rawMsg `json:"items"`
	rawMsg
}

type rawMsg struct {
	Unparseable bool              `json:"unparseable"`
	Method      string            `json:"method"`
	ID          json.RawMessage   `json:"id"`
	IsResponse  bool              `json:"is_response"`
	ParamsFull  json.RawMessage   `json:"params_full"`
	Result      *rawResultSummary `json:"result_summary"`
	ResultFull  json.RawMessage   `json:"result_full"`
	Error       *RPCError         `json:"error"`
}

type rawResultSummary struct {
	Bytes         int   `json:"bytes"`
	ContentBlocks *int  `json:"content_blocks"`
	TextLen       *int  `json:"text_len"`
	IsError       *bool `json:"is_error"`
}

// pendingReq is a request awaiting its response, keyed by the direction the
// response will travel plus the JSON-RPC id.
type pendingReq struct {
	method  string
	ts      string
	toolIdx int // index into Report.ToolCalls, or -1
}

type state struct {
	rep      *Report
	counter  Counter
	pending  map[string]*pendingReq
	methods  map[string]*MethodStat
	versions map[string]bool

	lastCallParams string
	haveLastCall   bool

	toolResponses            int
	toolResultTokens         int
	toolResultTokensStripped int
	fullResults              int

	warnOverflow int
}

// Analyze reads an interposer JSONL log and returns the per-trial report.
//
// The log is read as one trial. An append-only log with more than one header
// line (interposer/README.md warns this is possible) is still analyzed, with
// the session count reported, so the operator sees the mistake rather than a
// silently merged number.
func Analyze(r io.Reader, opts Options) (*Report, error) {
	counter := opts.Counter
	encoding := opts.Encoding
	if counter == nil {
		counter = tokens.NewOpenAI()
		if encoding == "" {
			encoding = tokens.OpenAIEncoding
		}
	}
	if encoding == "" {
		encoding = "custom"
	}

	st := &state{
		rep: &Report{
			AnalyzerVersion: Version,
			Source:          opts.Source,
			Tokenizer:       encoding,
			ToolCalls:       []ToolCall{},
			Methods:         []MethodStat{},
		},
		counter:  counter,
		pending:  map[string]*pendingReq{},
		methods:  map[string]*MethodStat{},
		versions: map[string]bool{},
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rl rawLine
		if err := json.Unmarshal(line, &rl); err != nil {
			st.rep.Totals.UnparseableLines++
			st.warnf("line %d: not valid JSON, skipped", lineNo)
			continue
		}
		switch {
		case rl.InterposerVersion != "":
			st.header(rl)
		case rl.Dir != "":
			if err := st.frame(rl, lineNo); err != nil {
				return nil, err
			}
		default:
			st.rep.Totals.UnrecognizedLines++
			st.warnf("line %d: neither a header nor a frame, skipped", lineNo)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	st.finish()
	return st.rep, nil
}

func (s *state) header(rl rawLine) {
	s.rep.Sessions++
	if !s.versions[rl.InterposerVersion] {
		s.versions[rl.InterposerVersion] = true
		s.rep.InterposerVersions = append(s.rep.InterposerVersions, rl.InterposerVersion)
	}
	if s.rep.InterposerVersion == "" {
		s.rep.InterposerVersion = rl.InterposerVersion
		s.rep.StartedAt = rl.StartedAt
		s.rep.ServerCmd = rl.ServerCmd
	}
}

func (s *state) frame(rl rawLine, lineNo int) error {
	t := &s.rep.Totals
	t.Frames++
	t.Bytes += rl.SizeBytes
	switch rl.Dir {
	case "c2s":
		t.FramesC2S++
		t.BytesC2S += rl.SizeBytes
	case "s2c":
		t.FramesS2C++
		t.BytesS2C += rl.SizeBytes
	default:
		s.warnf("line %d: unknown direction %q", lineNo, rl.Dir)
	}
	if rl.TS != "" {
		if s.rep.Wall.FirstFrame == "" {
			s.rep.Wall.FirstFrame = rl.TS
		}
		s.rep.Wall.LastFrame = rl.TS
	}

	if rl.Batch {
		t.BatchFrames++
		t.BatchMessages += len(rl.Items)
		for _, it := range rl.Items {
			if err := s.message(rl, it, lineNo); err != nil {
				return err
			}
		}
		return nil
	}
	return s.message(rl, rl.rawMsg, lineNo)
}

func (s *state) message(fr rawLine, m rawMsg, lineNo int) error {
	t := &s.rep.Totals
	if m.Unparseable {
		t.UnparseableFrames++
		return nil
	}
	switch {
	case m.IsResponse:
		t.Responses++
		s.response(fr, m, compactID(m.ID), lineNo)
		return nil
	case m.Method != "":
		return s.outbound(fr, m, lineNo)
	default:
		t.UnrecognizedLines++
		s.warnf("line %d: message carries neither a method nor a response payload", lineNo)
		return nil
	}
}

// outbound records a request or a notification: everything with a method.
// They share the params accounting; only a request registers a correlation.
func (s *state) outbound(fr rawLine, m rawMsg, lineNo int) error {
	t := &s.rep.Totals
	ms := s.methodStat(m.Method)

	argBytes := len(m.ParamsFull)
	argTokens := 0
	if argBytes > 0 {
		n, err := s.counter.Count(string(m.ParamsFull))
		if err != nil {
			return fmt.Errorf("line %d: count params tokens for %s: %w", lineNo, m.Method, err)
		}
		argTokens = n
	}
	ms.ParamsBytes += argBytes
	ms.ParamsTokens += argTokens

	if len(m.ID) == 0 {
		t.Notifications++
		ms.Notifications++
		return nil
	}
	t.Requests++
	ms.Requests++

	toolIdx := -1
	if m.Method == "tools/call" {
		canon := canonicalJSON(m.ParamsFull)
		retry := s.haveLastCall && canon == s.lastCallParams
		s.lastCallParams, s.haveLastCall = canon, true
		if retry {
			t.Retries++
		}
		t.ToolCalls++
		t.ToolCallArgBytes += argBytes
		t.ToolCallArgTokens += argTokens
		s.rep.ToolCalls = append(s.rep.ToolCalls, ToolCall{
			Seq:       len(s.rep.ToolCalls) + 1,
			ID:        compactID(m.ID),
			Tool:      toolName(m.ParamsFull),
			TS:        fr.TS,
			ArgBytes:  argBytes,
			ArgTokens: argTokens,
			Retry:     retry,
		})
		toolIdx = len(s.rep.ToolCalls) - 1
	}

	// The response travels the other way, so the pending entry is keyed by
	// the direction it will arrive on. That keeps a client request id from
	// colliding with an unrelated server request id of the same value.
	key := opposite(fr.Dir) + "|" + compactID(m.ID)
	if _, exists := s.pending[key]; exists {
		t.DuplicateRequestID++
		s.warnf("line %d: request id %s reused before the earlier one was answered", lineNo, compactID(m.ID))
	}
	s.pending[key] = &pendingReq{method: m.Method, ts: fr.TS, toolIdx: toolIdx}
	return nil
}

func (s *state) response(fr rawLine, m rawMsg, id string, lineNo int) error {
	t := &s.rep.Totals
	key := fr.Dir + "|" + id
	req := s.pending[key]
	if req == nil {
		t.UnmatchedResponses++
		s.warnf("line %d: response id %s has no logged request", lineNo, id)
	} else {
		delete(s.pending, key)
	}

	method := "(unmatched)"
	if req != nil {
		method = req.method
	}
	ms := s.methodStat(method)
	ms.Responses++

	resultBytes := 0
	var blocks, textLen *int
	toolError := false
	if m.Result != nil {
		resultBytes = m.Result.Bytes
		blocks, textLen = m.Result.ContentBlocks, m.Result.TextLen
		toolError = m.Result.IsError != nil && *m.Result.IsError
	}
	ms.ResultBytes += resultBytes
	if toolError {
		t.ToolErrors++
		ms.ToolErrors++
	}
	if m.Error != nil {
		t.JSONRPCErrors++
		ms.Errors++
	}

	if req == nil || req.toolIdx < 0 {
		return nil
	}

	cr := &CallResponse{
		TS:            fr.TS,
		ResultBytes:   resultBytes,
		ContentBlocks: blocks,
		TextLen:       textLen,
		IsToolError:   toolError,
		RPCError:      m.Error,
		LatencyMS:     spanMS(req.ts, fr.TS),
	}
	s.toolResponses++
	t.ToolCallResultBytes += resultBytes
	if len(m.ResultFull) > 0 {
		n, err := s.counter.Count(string(m.ResultFull))
		if err != nil {
			return fmt.Errorf("line %d: count result tokens for id %s: %w", lineNo, id, err)
		}
		cr.ResultTokens = &n
		s.toolResultTokens += n
		ns := n
		if strippedRaw, had := stripTopLevelMeta(m.ResultFull); had {
			sn, err := s.counter.Count(string(strippedRaw))
			if err != nil {
				return fmt.Errorf("line %d: count meta-stripped result tokens for id %s: %w", lineNo, id, err)
			}
			ns = sn
		}
		cr.ResultTokensMetaStripped = &ns
		s.toolResultTokensStripped += ns
		s.fullResults++
	}
	s.rep.ToolCalls[req.toolIdx].Response = cr
	return nil
}

// stripTopLevelMeta returns raw with the top-level _meta member of a JSON
// object removed, and whether such a member existed. Only the top level is
// touched, because that is where the MCP result envelope rides; a _meta
// nested anywhere deeper is the server's own payload and stays. The rebuild
// keeps every remaining member's value bytes verbatim and re-joins them
// compactly, which on wire-compact frames is byte-identical to deleting the
// member; when raw is not a JSON object or does not parse, it is returned
// unchanged so the stripped count degrades to the raw count rather than to
// an estimate.
func stripTopLevelMeta(raw []byte) ([]byte, bool) {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, err := d.Token()
	if err != nil {
		return raw, false
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return raw, false
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	found, first := false, true
	for d.More() {
		keyTok, err := d.Token()
		if err != nil {
			return raw, false
		}
		key, ok := keyTok.(string)
		if !ok {
			return raw, false
		}
		var val json.RawMessage
		if err := d.Decode(&val); err != nil {
			return raw, false
		}
		if key == "_meta" {
			found = true
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		kb, err := json.Marshal(key)
		if err != nil {
			return raw, false
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(val)
	}
	if !found {
		return raw, false
	}
	buf.WriteByte('}')
	return buf.Bytes(), true
}

func (s *state) finish() {
	t := &s.rep.Totals
	t.UnmatchedRequests = len(s.pending)

	if s.toolResponses > 0 && s.fullResults == s.toolResponses {
		total := s.toolResultTokens
		t.ToolCallResultTokens = &total
		strippedTotal := s.toolResultTokensStripped
		t.ToolCallResultTokensMetaStripped = &strippedTotal
	} else if s.toolResponses > 0 {
		s.warnf("result tokens unavailable: %d of %d tool-call responses carry result_full; rerun the interposer with --full-results to measure them",
			s.fullResults, s.toolResponses)
	}

	s.rep.Wall.DurationMS = spanMS(s.rep.Wall.FirstFrame, s.rep.Wall.LastFrame)

	if len(s.rep.InterposerVersions) > 1 {
		s.warnf("log carries %d interposer versions (%v); frames from different versions are not comparable and this report is not publishable",
			len(s.rep.InterposerVersions), s.rep.InterposerVersions)
	} else {
		s.rep.InterposerVersions = nil
	}
	if s.rep.Sessions == 0 {
		s.warnf("log has no header line, so interposer_version is unknown; section 5 withholds a result missing a version stamp")
	} else if s.rep.Sessions > 1 {
		s.warnf("log contains %d sessions; one log per trial is the section 6 convention", s.rep.Sessions)
	}

	for _, ms := range s.methods {
		s.rep.Methods = append(s.rep.Methods, *ms)
	}
	sort.Slice(s.rep.Methods, func(i, j int) bool {
		return s.rep.Methods[i].Method < s.rep.Methods[j].Method
	})

	if s.warnOverflow > 0 {
		s.rep.Warnings = append(s.rep.Warnings, fmt.Sprintf("%d further warnings suppressed", s.warnOverflow))
	}
}

func (s *state) methodStat(method string) *MethodStat {
	if ms, ok := s.methods[method]; ok {
		return ms
	}
	ms := &MethodStat{Method: method}
	s.methods[method] = ms
	return ms
}

func (s *state) warnf(format string, args ...any) {
	if len(s.rep.Warnings) >= maxWarnings {
		s.warnOverflow++
		return
	}
	s.rep.Warnings = append(s.rep.Warnings, fmt.Sprintf(format, args...))
}

// compactID renders a JSON-RPC id as a stable string. The id is preserved as
// sent, so 1 and "1" are different ids and must not collapse onto one key.
func compactID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// canonicalJSON normalizes a params object for the retry comparison, so that
// two calls differing only in key order or whitespace count as equivalent.
// Go marshals map keys in sorted order, which is what makes the round trip
// canonical.
func canonicalJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func toolName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var p struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.Name
}

func opposite(dir string) string {
	if dir == "c2s" {
		return "s2c"
	}
	return "c2s"
}

// spanMS returns the milliseconds between two RFC3339Nano stamps, or nil if
// either is missing or unparseable.
func spanMS(from, to string) *int64 {
	if from == "" || to == "" {
		return nil
	}
	a, err := time.Parse(time.RFC3339Nano, from)
	if err != nil {
		return nil
	}
	b, err := time.Parse(time.RFC3339Nano, to)
	if err != nil {
		return nil
	}
	ms := b.Sub(a).Milliseconds()
	return &ms
}
