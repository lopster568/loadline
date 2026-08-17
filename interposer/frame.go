package interposer

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// logger appends one JSON object per line to the frame log. Writes are
// unbuffered: each record reaches the operating system in a single write
// syscall, so a crash of this process cannot lose an already-relayed frame.
type logger struct {
	mu   sync.Mutex
	w    io.Writer
	full bool
}

// header is the first line of every log file. It pins the interposer version,
// because measurements are only comparable within one version.
type header struct {
	InterposerVersion string   `json:"interposer_version"`
	StartedAt         string   `json:"started_at"`
	ServerCmd         []string `json:"server_cmd"`
	PID               int      `json:"pid"`
}

// record is one wire frame. size_bytes counts the frame exactly as it crossed
// the pipe, trailing newline included.
type record struct {
	TS        string `json:"ts"`
	Dir       string `json:"dir"`
	SizeBytes int    `json:"size_bytes"`
	Batch     bool   `json:"batch,omitempty"`
	BatchLen  int    `json:"batch_len,omitempty"`
	Items     []msg  `json:"items,omitempty"`
	msg
}

// msg holds the JSON-RPC fields of a single message. A frame carrying one
// message promotes these to the top level; a batch frame nests one per item.
type msg struct {
	Unparseable bool            `json:"unparseable,omitempty"`
	Method      string          `json:"method,omitempty"`
	ID          json.RawMessage `json:"id,omitempty"`
	IsResponse  bool            `json:"is_response,omitempty"`
	ParamsFull  json.RawMessage `json:"params_full,omitempty"`
	Result      *resultSummary  `json:"result_summary,omitempty"`
	ResultFull  json.RawMessage `json:"result_full,omitempty"`
	Error       *rpcError       `json:"error,omitempty"`
	ErrorFull   json.RawMessage `json:"error_full,omitempty"`
}

type resultSummary struct {
	Bytes         int   `json:"bytes"`
	ContentBlocks *int  `json:"content_blocks,omitempty"`
	TextLen       *int  `json:"text_len,omitempty"`
	IsError       *bool `json:"is_error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// wire is the subset of JSON-RPC we read. RawMessage distinguishes an absent
// member from a null one, which is how a response is told from a request.
type wire struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (l *logger) write(v any) {
	line, err := json.Marshal(v)
	if err != nil {
		return
	}
	line = append(line, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(line)
}

func (l *logger) header(serverCmd []string, pid int) {
	l.write(&header{
		InterposerVersion: Version,
		StartedAt:         stamp(),
		ServerCmd:         serverCmd,
		PID:               pid,
	})
}

// frame logs one relayed frame. Anything that is not a JSON-RPC message is
// recorded as unparseable; it has already been relayed either way.
func (l *logger) frame(dir string, raw []byte) {
	r := record{TS: stamp(), Dir: dir, SizeBytes: len(raw)}
	body := bytes.TrimSpace(raw)

	switch {
	case len(body) == 0:
		r.Unparseable = true
	case body[0] == '[':
		var items []json.RawMessage
		if json.Unmarshal(body, &items) != nil {
			r.Unparseable = true
			break
		}
		r.Batch = true
		r.BatchLen = len(items)
		r.Items = make([]msg, 0, len(items))
		for _, item := range items {
			r.Items = append(r.Items, l.decode(item))
		}
	default:
		r.msg = l.decode(body)
	}
	l.write(&r)
}

func (l *logger) decode(raw []byte) msg {
	var w wire
	if json.Unmarshal(raw, &w) != nil {
		return msg{Unparseable: true}
	}
	m := msg{Method: w.Method, ID: w.ID, ParamsFull: w.Params}
	if len(w.Result) > 0 {
		m.IsResponse = true
		m.Result = summarize(w.Result)
		if l.full {
			m.ResultFull = w.Result
		}
	}
	if len(w.Error) > 0 {
		m.IsResponse = true
		var e rpcError
		if json.Unmarshal(w.Error, &e) == nil {
			m.Error = &e
		}
		if l.full {
			m.ErrorFull = w.Error
		}
	}
	return m
}

// summarize describes a response result without copying it. For a tools/call
// result, which carries a content array, it also reports the block count and
// the total UTF-8 byte length of the text blocks.
func summarize(result []byte) *resultSummary {
	s := &resultSummary{Bytes: len(result)}
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError *bool `json:"isError"`
	}
	if json.Unmarshal(result, &r) != nil {
		return s
	}
	s.IsError = r.IsError
	if r.Content != nil {
		blocks, textLen := len(r.Content), 0
		for _, c := range r.Content {
			textLen += len(c.Text)
		}
		s.ContentBlocks = &blocks
		s.TextLen = &textLen
	}
	return s
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }
