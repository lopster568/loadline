// Package sweep runs the Tier 1 static sweep: acquire, negotiate, enumerate,
// canonicalize, count. A failure is an output row, never a stopped sweep.
package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lopster568/loadline/internal/canon"
	"github.com/lopster568/loadline/internal/corpus"
	"github.com/lopster568/loadline/internal/hygiene"
	"github.com/lopster568/loadline/internal/mcpwire"
	"github.com/lopster568/loadline/internal/modes"
	"github.com/lopster568/loadline/internal/report"
	"github.com/lopster568/loadline/internal/retrieval"
	"github.com/lopster568/loadline/internal/tokens"
)

// Config is the harness run configuration. The per-step budget is recorded so
// a timeout row can cite the budget it exceeded (methodology 7).
type Config struct {
	StepTimeout   time.Duration
	ServerTimeout time.Duration
	ClaudeModel   string
	GeminiModel   string
	ScratchDir    string
	RefPolicy     canon.RefPolicy
	ClientInfo    mcpwire.ClientInfo
	Logf          func(format string, args ...any)
}

// DefaultConfig returns the v0 budgets and pins.
func DefaultConfig() Config {
	return Config{
		StepTimeout:   60 * time.Second,
		ServerTimeout: 240 * time.Second,
		ClaudeModel:   tokens.DefaultClaudeModel,
		GeminiModel:   tokens.DefaultGeminiModel,
		RefPolicy:     canon.DefaultRefPolicy(),
		ClientInfo: mcpwire.ClientInfo{
			Name:    "loadline",
			Title:   "loadline measurement harness",
			Version: report.HarnessVersion,
		},
		Logf: func(string, ...any) {},
	}
}

// offlineCounter is the local o200k_base tokenizer. It is an interface so the
// failed-count path stays testable: with the embedded BPE ranks in the binary
// a real counter has no reachable failure mode, and the path it leaves behind
// is exactly the one that must not publish a measured zero.
type offlineCounter interface {
	Count(text string) (int, error)
}

// Runner executes the sweep.
type Runner struct {
	cfg    Config
	openai offlineCounter
	claude *tokens.Claude
	gemini *tokens.Gemini
}

// NewRunner wires the tokenizer adapters.
func NewRunner(cfg Config) *Runner {
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	return &Runner{
		cfg:    cfg,
		openai: tokens.NewOpenAI(),
		claude: tokens.NewClaude(cfg.ClaudeModel),
		gemini: tokens.NewGemini(cfg.GeminiModel),
	}
}

// Run sweeps the given corpus entries and returns one row per entry.
func (r *Runner) Run(ctx context.Context, servers []corpus.Server) *report.Document {
	doc := &report.Document{
		SchemaVersion: report.SchemaVersion,
		Sample:        false,
		Run: report.Run{
			Date:               time.Now().UTC().Format("2006-01-02"),
			MethodologyVersion: report.MethodologyVersion,
			HarnessVersion:     report.HarnessVersion,
		},
	}
	for _, s := range servers {
		// An operator abort is not a measurement outcome. Methodology 7's
		// failure classes are all server-attributable, so a server the sweep
		// never reached, or was cut short on, publishes no row at all; the run
		// is stamped aborted instead, which keeps the completed rows publishable
		// (a failure never blocks a release) without inventing a status for a
		// server nothing was learned about.
		if ctx.Err() != nil {
			doc.Run.Aborted = true
			break
		}
		row := r.runOne(ctx, s)
		if row.Status == statusAborted {
			doc.Run.Aborted = true
			break
		}
		doc.Servers = append(doc.Servers, row)
	}
	return doc
}

func (r *Runner) runOne(ctx context.Context, s corpus.Server) (row report.Server) {
	row = report.Server{
		ID:             s.ID,
		Name:           s.Name,
		Maintainer:     maintainer(s),
		Category:       s.Category,
		Status:         report.StatusOK,
		Counts:         r.emptyCounts(),
		CorpusNotes:    s.Notes(),
		MeasuredAt:     time.Now().UTC(),
		MethodologyVer: report.MethodologyVersion,
		HarnessVer:     report.HarnessVersion,
	}

	// A panic in one server's path must not end the sweep.
	defer func() {
		if p := recover(); p != nil {
			row.Status = report.StatusProtocolError
			row.Error = fmt.Sprintf("harness panic: %v", p)
		}
	}()

	serverCtx, cancel := context.WithTimeout(ctx, r.cfg.ServerTimeout)
	defer cancel()

	l, err := resolveLaunch(s, r.cfg.ScratchDir)
	if err != nil {
		row.Status = report.StatusUnreachable
		row.Error = err.Error()
		return row
	}
	envName, envValue, err := credential(s)
	if err != nil {
		row.Status = report.StatusAuth
		row.Error = err.Error()
		row.Acquisition = acquisitionOf(l, nil)
		return row
	}

	var envPassed []string
	if envName != "" && envValue != "" {
		envPassed = []string{envName}
	}
	row.Acquisition = acquisitionOf(l, envPassed)

	tr, err := r.dial(serverCtx, l, envName, envValue)
	if err != nil {
		row.Status, row.Error = classify(serverCtx, err)
		return row
	}
	defer tr.Close()

	client := mcpwire.NewClient(tr, r.cfg.ClientInfo)

	negCtx, negCancel := context.WithTimeout(serverCtx, r.cfg.StepTimeout)
	neg, err := client.Negotiate(negCtx)
	negCancel()
	if err != nil {
		row.Status, row.Error = classify(serverCtx, err)
		if diag := client.Diagnostics(); diag != "" {
			row.Error += " | server stderr: " + firstLines(diag, 5)
		}
		return row
	}
	row.ProtocolRevision = neg.Revision
	row.Negotiation = &report.Negotiation{Branch: neg.Branch, SupportedVersions: neg.SupportedVersions}
	row.Provenance.ServerVersion = neg.ServerInfo.Version

	listCtx, listCancel := context.WithTimeout(serverCtx, r.cfg.StepTimeout)
	enum, err := client.ListTools(listCtx)
	listCancel()
	if err != nil {
		if enum != nil && enum.Partial {
			row.Status = report.StatusPartialSurface
			row.Error = err.Error()
			return row
		}
		row.Status, row.Error = classify(serverCtx, err)
		return row
	}

	surface, err := canon.Canonicalize(enum.RawTools, enum.Pages, r.cfg.RefPolicy)
	if surface != nil {
		row.Provenance.WireSHA256 = surface.WireSHA256
		row.SchemaFlags = surface.Flags
		row.ExcludedTools = surface.Excluded
	}
	if err != nil {
		if errors.Is(err, canon.ErrSchemaInvalid) {
			row.Status = report.StatusSchemaInvalid
			row.Error = surface.SchemaInvalidWhy
		} else {
			row.Status = report.StatusProtocolError
			row.Error = err.Error()
		}
		return row
	}

	row.Provenance.CanonicalSHA256 = surface.CanonicalSHA256
	row.Provenance.SortedSHA256 = surface.SortedSHA256
	counted := surface.Counted()
	row.ToolCount = len(counted)

	// Retrievability and hygiene are derived from the surface already in hand,
	// so they cost no further contact with the server and do not depend on any
	// token count succeeding. A surface with no counted tools is scored as
	// neither: there is nothing to retrieve and nothing to grade, and a zero in
	// either field would read as a measured failure of a server that published
	// no tools rather than as the absence it is. hygiene.Compute enforces the
	// same rule on its own side by returning nil; the guard here stays as belt
	// and braces and because retrievability has no equivalent.
	if len(counted) > 0 {
		res := retrieval.Evaluate(s.ID, retrievalTools(counted))
		row.Retrievability = &res.Score
		row.Queries = &res.Set
		row.Hygiene = hygiene.Compute(hygieneTools(counted))
	}

	// Enumeration can succeed while counting fails. That is still status ok,
	// because nothing about the server failed, but the row then carries no mode
	// figures: modes are computed on the o200k_base count (methodology 3.1), and
	// a missing count must publish as a missing block, never as a measured zero.
	// Passing 0 into modes.Compute would publish naive {tokens:0,
	// kind:"measured"} and a code-mode estimate of 1000 exceeding it, which
	// contradicts the clamp of methodology 3.3.
	perToolTokens, openaiOK := r.countCells(serverCtx, surface, counted, &row)
	if openaiOK {
		set := modes.Compute(row.Counts.OpenAI.TotalSchemaTokens, perToolTokens)
		row.Modes = &set
	}
	return row
}

// countCells fills the three provider cells and reports whether the o200k_base
// cell is a complete measurement, which is the precondition for publishing any
// mode figure.
func (r *Runner) countCells(ctx context.Context, surface *canon.Surface, counted []canon.Tool, row *report.Server) ([]int, bool) {
	oa := report.OpenAICell{PerTool: map[string]int{}, Encoding: tokens.OpenAIEncoding}
	var perToolTokens []int
	total, err := r.openai.Count(surface.Canonical)
	if err != nil {
		oa.Error = err.Error()
	} else {
		complete := true
		for _, t := range counted {
			n, cerr := r.openai.Count(t.Canonical)
			if cerr != nil {
				// A partial per-tool set would make the tool-search average a
				// figure over an arbitrary prefix of the surface, so the whole
				// cell is withheld rather than published half measured.
				oa.Error = cerr.Error()
				complete = false
				break
			}
			oa.PerTool[t.Name] = n
			perToolTokens = append(perToolTokens, n)
		}
		if complete {
			oa.Available = true
			oa.TotalSchemaTokens = total
			oa.MeasuredAt = time.Now().UTC().Format(time.RFC3339)
		} else {
			oa.PerTool = map[string]int{}
			perToolTokens = nil
		}
	}
	row.Counts.OpenAI = oa

	cl := report.ClaudeCell{Model: r.claude.Model}
	if r.claude.Available() {
		n, err := r.claude.Count(ctx, surface.Canonical)
		if err != nil {
			cl.Error = err.Error()
		} else {
			cl.Available = true
			cl.TotalSchemaTokens = n
			cl.MeasuredAt = time.Now().UTC().Format(time.RFC3339)
			native, nerr := r.claude.CountNativeTools(ctx, anthropicTools(counted))
			if nerr != nil {
				cl.Error = "native tools-parameter count: " + nerr.Error()
			} else {
				cl.NativeToolsParamTokens = native
			}
		}
	} else {
		cl.Error = "ANTHROPIC_API_KEY not set"
	}
	row.Counts.Claude = cl

	gm := report.GeminiCell{Model: r.gemini.Model}
	if r.gemini.Available() {
		n, err := r.gemini.Count(ctx, surface.Canonical)
		if err != nil {
			gm.Error = err.Error()
		} else {
			gm.Available = true
			gm.TotalSchemaTokens = n
			gm.MeasuredAt = time.Now().UTC().Format(time.RFC3339)
		}
	} else {
		gm.Error = "GEMINI_API_KEY not set"
	}
	row.Counts.Gemini = gm

	return perToolTokens, row.Counts.OpenAI.Available
}

func (r *Runner) emptyCounts() report.Counts {
	return report.Counts{
		OpenAI: report.OpenAICell{PerTool: map[string]int{}, Encoding: tokens.OpenAIEncoding},
		Claude: report.ClaudeCell{Model: r.cfg.ClaudeModel},
		Gemini: report.GeminiCell{Model: r.cfg.GeminiModel},
	}
}

func (r *Runner) dial(ctx context.Context, l launch, envName, envValue string) (mcpwire.Transport, error) {
	switch l.transport {
	case "stdio":
		env := inheritedEnv()
		if envName != "" && envValue != "" {
			env = append(env, envName+"="+envValue)
		}
		dialCtx, cancel := context.WithTimeout(ctx, r.cfg.StepTimeout)
		defer cancel()
		return mcpwire.DialStdio(dialCtx, mcpwire.StdioConfig{
			Command: l.command,
			Args:    l.args,
			Env:     env,
		})
	case "remote":
		headers := map[string]string{}
		if envValue != "" {
			headers["Authorization"] = "Bearer " + envValue
		}
		return mcpwire.DialHTTP(mcpwire.HTTPConfig{
			Endpoint: l.endpoint,
			Headers:  headers,
			Client:   &http.Client{Timeout: r.cfg.StepTimeout},
		}), nil
	}
	return nil, fmt.Errorf("unsupported transport %q", l.transport)
}

// inheritedEnv is the recorded environment for a stdio launch: the minimum a
// package runner needs, plus the server's own credential added by the caller.
func inheritedEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "HOME", "USER", "LANG", "LC_ALL", "TMPDIR", "SHELL", "XDG_CACHE_HOME", "XDG_DATA_HOME", "NODE_PATH", "NPM_CONFIG_CACHE", "UV_CACHE_DIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// retrievalTools and hygieneTools project the canonical surface onto the two
// metric packages' inputs. Both packages take their own input type rather than
// canon.Tool so a change to the canonicalizer cannot silently change what a
// published metric is computed over. The descriptor is carried across rather
// than rebuilt on either side: methodology 5 names the canonicalizer's join as
// the indexed document, and dimension 6 of methodology 6 tokenizes the same
// string, so both metrics read one descriptor per tool.
func retrievalTools(counted []canon.Tool) []retrieval.Tool {
	out := make([]retrieval.Tool, 0, len(counted))
	for _, t := range counted {
		out = append(out, retrieval.Tool{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			Descriptor:  t.Descriptor,
		})
	}
	return out
}

func hygieneTools(counted []canon.Tool) []hygiene.Tool {
	out := make([]hygiene.Tool, 0, len(counted))
	for _, t := range counted {
		out = append(out, hygiene.Tool{
			Name:            t.Name,
			Title:           t.Title,
			Description:     t.Description,
			Descriptor:      t.Descriptor,
			InputSchemaJSON: t.InputSchemaJSON,
		})
	}
	return out
}

func anthropicTools(counted []canon.Tool) []tokens.AnthropicTool {
	out := make([]tokens.AnthropicTool, 0, len(counted))
	for _, t := range counted {
		schema := t.InputSchemaJSON
		if schema == "" {
			schema = `{"type":"object","properties":{}}`
		}
		out = append(out, tokens.AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: json.RawMessage(schema),
		})
	}
	return out
}

func acquisitionOf(l launch, envPassed []string) *report.Acquisition {
	return &report.Acquisition{
		Transport:  l.transport,
		Source:     l.source,
		Command:    l.command,
		Args:       l.recorded,
		Endpoint:   l.endpoint,
		EnvPassed:  envPassed,
		Pinned:     l.pinned,
		AcquiredAt: time.Now().UTC(),
	}
}

func maintainer(s corpus.Server) string {
	if s.MaintainerType == "official" {
		return "official"
	}
	return "community"
}

// statusAborted is deliberately not one of report's status constants.
// Methodology 7 enumerates failure classes that are all properties of the
// server under measurement; an operator pressing Ctrl-C is a property of the
// run. A row carrying it is dropped by Run rather than published, so no server
// is ever labelled with a failure the sweep caused itself.
const statusAborted = "aborted"

// classify maps a harness error onto a methodology 7 failure class, or onto
// statusAborted when the sweep was cancelled rather than timed out. The two
// look identical at the call site (both surface as a non-nil ctx.Err()) and
// must not: context.DeadlineExceeded is a genuine, publishable exceeded budget,
// while context.Canceled says nothing measurable about the server.
func classify(ctx context.Context, err error) (string, string) {
	if err == nil {
		return report.StatusOK, ""
	}
	detail := err.Error()

	var authErr *mcpwire.AuthError
	if errors.As(err, &authErr) {
		return report.StatusAuth, detail
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return report.StatusTimeout, detail
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return statusAborted, detail
	}
	var protoErr *mcpwire.ProtocolError
	if errors.As(err, &protoErr) {
		return report.StatusProtocolError, detail
	}
	var rpcErr *mcpwire.RPCError
	if errors.As(err, &rpcErr) {
		if looksLikeAuth(rpcErr.Message) {
			return report.StatusAuth, detail
		}
		return report.StatusProtocolError, detail
	}
	lower := strings.ToLower(detail)
	switch {
	case strings.Contains(lower, "executable file not found"),
		strings.Contains(lower, "no such file or directory"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "closed stdout before responding"),
		strings.Contains(lower, "broken pipe"),
		strings.Contains(lower, "file already closed"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "launch "):
		return report.StatusUnreachable, detail
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "deadline exceeded"):
		return report.StatusTimeout, detail
	case looksLikeAuth(lower):
		return report.StatusAuth, detail
	}
	return report.StatusProtocolError, detail
}

func looksLikeAuth(s string) bool {
	lower := strings.ToLower(s)
	for _, marker := range []string{"unauthorized", "unauthenticated", "forbidden", "invalid api key", "invalid token", "authentication"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " | ")
}
