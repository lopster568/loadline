// Package report defines the published run schema and writes run artifacts.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lopster568/loadline/internal/hygiene"
	"github.com/lopster568/loadline/internal/modes"
	"github.com/lopster568/loadline/internal/retrieval"
)

// SchemaVersion is the version of the published JSON shape.
const SchemaVersion = "0.3"

// MethodologyVersion tracks docs/methodology-v0.md.
const MethodologyVersion = "0.3.1"

// HarnessVersion is stamped on every cell. Results are never compared across
// harness versions (methodology 9).
const HarnessVersion = "0.1.0"

// Status values. partial_surface comes from methodology 7 and is the class for
// an enumeration that started but could not complete.
const (
	StatusOK             = "ok"
	StatusUnreachable    = "unreachable"
	StatusAuth           = "auth"
	StatusProtocolError  = "protocol_error"
	StatusTimeout        = "timeout"
	StatusPartialSurface = "partial_surface"
	StatusSchemaInvalid  = "schema_invalid"
)

// Document is one run artifact.
type Document struct {
	SchemaVersion string   `json:"schema_version"`
	Sample        bool     `json:"sample"`
	Run           Run      `json:"run"`
	Servers       []Server `json:"servers"`
}

// Run stamps the sweep.
type Run struct {
	Date               string `json:"date"`
	MethodologyVersion string `json:"methodology_version"`
	HarnessVersion     string `json:"harness_version"`
	// Aborted marks a sweep the operator cut short. Methodology 7 enumerates
	// server-attributable failure classes only, so an abort is a property of
	// the run rather than of any server, and the servers it never reached
	// carry no row at all.
	Aborted bool `json:"aborted,omitempty"`
}

// Server is one published row. A failure row carries the same stamp fields as
// a successful one, minus token counts, plus the failure class and raw error.
type Server struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Maintainer       string     `json:"maintainer"`
	Status           string     `json:"status"`
	ToolCount        int        `json:"tool_count"`
	ProtocolRevision string     `json:"protocol_revision"`
	Provenance       Provenance `json:"provenance"`
	Counts           Counts     `json:"counts"`

	// Modes is nil, and publishes as JSON null, whenever the o200k_base count
	// the mode formulas of methodology 3 are computed on is unavailable. An
	// unmeasured value must never publish as a measured zero, so the absence of
	// a figure is published as an absence rather than as 0.
	Modes *modes.Set `json:"modes"`

	// Retrievability and Hygiene are computed from the enumerated surface
	// alone (methodology 5 and 6), so they survive a token-count failure and
	// are absent only when there was no surface to score. Like Modes they
	// publish as JSON null rather than as a zero: a zero retrievability or a
	// zero hygiene score is a damning measurement, and a server that was never
	// reached has not earned it.
	Retrievability *retrieval.Score `json:"retrievability"`
	Hygiene        *hygiene.Grade   `json:"hygiene"`

	// Queries is the derivation record behind Retrievability. It is written to
	// its own artifact rather than inlined into the row, because it is bulky
	// and its audience is a critic reproducing the score rather than the
	// calculator rendering it.
	Queries *retrieval.QuerySet `json:"-"`

	// WirePages carries the raw tools/list wire bytes, one entry per page, in
	// the exact order and exact bytes canon.WireHash was computed over. It is
	// written to its own artifact rather than inlined into the row for the
	// same bulk reason as Queries, but its purpose is different: the site
	// promises maintainers the raw run artifact backing WireSHA256, and a hash
	// alone is not that artifact, only its digest. Nil whenever enumeration
	// never happened, which is every status short of a canon.Canonicalize call.
	WirePages [][]byte `json:"-"`

	// Fields below are additive to the site contract and carry the run-record
	// obligations of methodology 1.1, 1.3, 7 and 8.
	Category       string       `json:"category,omitempty"`
	Error          string       `json:"error,omitempty"`
	Acquisition    *Acquisition `json:"acquisition,omitempty"`
	Negotiation    *Negotiation `json:"negotiation,omitempty"`
	SchemaFlags    []string     `json:"schema_flags,omitempty"`
	ExcludedTools  []string     `json:"excluded_tools,omitempty"`
	CorpusNotes    []string     `json:"corpus_notes,omitempty"`
	MeasuredAt     time.Time    `json:"measured_at"`
	MethodologyVer string       `json:"methodology_version"`
	HarnessVer     string       `json:"harness_version"`
}

// Provenance carries the two anti-gaming hashes and the server pin.
type Provenance struct {
	ServerVersion   string `json:"server_version"`
	WireSHA256      string `json:"wire_sha256"`
	CanonicalSHA256 string `json:"canonical_sha256"`
	// SortedSHA256 is the key-order-insensitive digest. Methodology 1.5 fixes
	// the canonical string at the server's own key order, while methodology 8
	// wants a digest that tolerates cosmetic reordering; this field carries the
	// second property without disturbing the first.
	SortedSHA256 string `json:"canonical_sorted_sha256,omitempty"`
}

// Acquisition records the pin and source required by methodology 1.1.
type Acquisition struct {
	Transport  string    `json:"transport"`
	Source     string    `json:"source"`
	Command    string    `json:"command,omitempty"`
	Args       []string  `json:"args,omitempty"`
	Endpoint   string    `json:"endpoint,omitempty"`
	EnvPassed  []string  `json:"env_passed,omitempty"`
	Pinned     bool      `json:"pinned"`
	AcquiredAt time.Time `json:"acquired_at"`
	// ResolvedDeps is what the resolver produced for this acquisition, which
	// Pinned does not answer: Pinned says whether the server package itself
	// carried a version, and says nothing about the dependency versions the
	// resolver chose underneath it. A server declaring an unbounded dependency
	// on the MCP SDK can start on one resolve and die at import on the next
	// with an identical Pinned value, so a row without this field cannot
	// explain its own failure (methodology 1.1).
	ResolvedDeps *ResolvedDeps `json:"resolved_deps"`
}

// ResolvedDeps is the dependency set one acquisition resolved to. Method and
// Command say how it was read, so a reader can rerun the same listing; Env is
// the environment it was read from, which for a package runner is the same
// directory the server process runs out of and therefore checkable against a
// path in a traceback.
type ResolvedDeps struct {
	Method  string   `json:"method"`
	Command []string `json:"command,omitempty"`
	Env     string   `json:"env,omitempty"`
	// SDK is the MCP SDK entry lifted out of Packages, because that is the one
	// version a reader of an unreachable row looks for first. It is null when
	// the resolved set carries no SDK package, which includes every acquisition
	// kind that resolves nothing locally.
	SDK      *DepPackage  `json:"sdk"`
	Packages []DepPackage `json:"packages,omitempty"`
	// Note carries the reason when there is no listing to record. A bare null
	// would say a listing is missing without saying why it was never possible.
	Note string `json:"note,omitempty"`
	// Error records a listing that was attempted and failed. A failed listing
	// is data on the row, never a failed acquisition: the server is measured
	// either way.
	Error string `json:"error,omitempty"`
}

// DepPackage is one resolved distribution: the name as its registry spells it
// and the version the resolver returned.
type DepPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Negotiation records which revision branch the probe took.
type Negotiation struct {
	Branch            string   `json:"branch"`
	SupportedVersions []string `json:"supported_versions,omitempty"`
}

// Counts holds one cell per provider.
type Counts struct {
	OpenAI OpenAICell `json:"openai_o200k"`
	Claude ClaudeCell `json:"claude"`
	Gemini GeminiCell `json:"gemini"`
}

// OpenAICell is the local tiktoken count.
type OpenAICell struct {
	TotalSchemaTokens int            `json:"total_schema_tokens"`
	PerTool           map[string]int `json:"per_tool"`
	Encoding          string         `json:"encoding,omitempty"`
	Available         bool           `json:"available"`
	MeasuredAt        string         `json:"measured_at,omitempty"`
	Error             string         `json:"error,omitempty"`
}

// ClaudeCell is the count_tokens result plus the native tools-parameter figure.
type ClaudeCell struct {
	Model                  string `json:"model"`
	Available              bool   `json:"available"`
	TotalSchemaTokens      int    `json:"total_schema_tokens"`
	NativeToolsParamTokens int    `json:"native_tools_param_tokens"`
	MeasuredAt             string `json:"measured_at,omitempty"`
	Error                  string `json:"error,omitempty"`
}

// GeminiCell is the countTokens result.
type GeminiCell struct {
	Model             string `json:"model"`
	Available         bool   `json:"available"`
	TotalSchemaTokens int    `json:"total_schema_tokens"`
	MeasuredAt        string `json:"measured_at,omitempty"`
	Error             string `json:"error,omitempty"`
}

// Write persists per-server artifacts under runs/<date>/ and the aggregate
// latest.json, both rooted at outDir.
func Write(outDir string, doc *Document) ([]string, error) {
	runDir := filepath.Join(outDir, "runs", doc.Run.Date)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, s := range doc.Servers {
		single := &Document{
			SchemaVersion: doc.SchemaVersion,
			Sample:        doc.Sample,
			Run:           doc.Run,
			Servers:       []Server{s},
		}
		path := filepath.Join(runDir, s.ID+".json")
		if err := writeJSON(path, single); err != nil {
			return written, err
		}
		written = append(written, path)

		// The derived query set is published so the retrievability score can be
		// inspected rather than taken on trust: mechanical derivation is only
		// an improvement on hand-written queries if the output is visible.
		if s.Queries != nil {
			qpath := filepath.Join(runDir, s.ID+"-queries.json")
			if err := writeJSON(qpath, s.Queries); err != nil {
				return written, err
			}
			written = append(written, qpath)
		}

		// The raw wire pages are published alongside the row so
		// provenance.wire_sha256 is a checkable claim rather than an assertion:
		// sha256(concat(pages)) over this file must reproduce the hash. Written
		// verbatim, one page per line, with no re-encoding, since byte fidelity
		// to what canon.WireHash actually hashed is the entire point.
		if len(s.WirePages) > 0 {
			wpath := filepath.Join(runDir, s.ID+"-wire.jsonl")
			if err := writeWireJSONL(wpath, s.WirePages); err != nil {
				return written, err
			}
			written = append(written, wpath)
		}
	}
	latest := filepath.Join(outDir, "latest.json")
	if err := writeJSON(latest, doc); err != nil {
		return written, err
	}
	written = append(written, latest)
	return written, nil
}

// writeWireJSONL writes the raw tools/list wire pages verbatim, one per line
// in wire order. Each page is the exact byte slice canon.WireHash concatenated
// and hashed (mcpwire reads it off a newline-delimited stdio scanner, so a
// page carries no embedded newline of its own), so the file is written
// byte-for-byte with no marshaling: a reader that strips the trailing '\n'
// from each line and concatenates the pages back together reproduces the
// exact bytes wire_sha256 was computed over.
func writeWireJSONL(path string, pages [][]byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer f.Close()
	for _, p := range pages {
		if _, err := f.Write(p); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
