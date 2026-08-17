// Package report defines the published run schema and writes run artifacts.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lopster568/loadline/internal/modes"
)

// SchemaVersion is the version of the published JSON shape.
const SchemaVersion = "0.1"

// MethodologyVersion tracks docs/methodology-v0.md.
const MethodologyVersion = "0.1.1"

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
	}
	latest := filepath.Join(outDir, "latest.json")
	if err := writeJSON(latest, doc); err != nil {
		return written, err
	}
	written = append(written, latest)
	return written, nil
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
