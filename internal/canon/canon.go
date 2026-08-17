package canon

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/lopster568/loadline/internal/retrieval"
)

// toolFieldAllow is the canonical tool-object field set from methodology 1.5.
// The section restricts the object to the first six fields and then states that
// "icons, _meta, and x-mcp-header annotations" are retained, so icons and _meta
// are kept at tool level and nothing is stripped inside a retained field.
var toolFieldAllow = []string{
	"name", "title", "description", "inputSchema", "outputSchema",
	"annotations", "icons", "_meta",
}

// Tool is one canonicalized tool from the surface.
type Tool struct {
	Name      string   `json:"name"`
	Canonical string   `json:"-"`
	Flags     []string `json:"flags,omitempty"`
	Excluded  bool     `json:"excluded,omitempty"`
	// Descriptor carries name, title, and description concatenated, which is
	// the text the retrievability index in methodology 5 is built over and the
	// token base of the disambiguation dimension in methodology 6. Both metric
	// packages consume this field rather than rejoining the parts, so there is
	// one descriptor per tool and it is this one.
	Descriptor string `json:"-"`
	// Title and Description carry the two prose fields separately, which the
	// retrievability and hygiene metrics of methodology 5 and 6 need to tell a
	// missing description from a missing title. Description and
	// InputSchemaJSON also feed the Anthropic tool-definition shape of
	// methodology 1.5, after $ref inlining.
	Title           string `json:"-"`
	Description     string `json:"-"`
	InputSchemaJSON string `json:"-"`
}

// Surface is the canonicalization result for a whole server.
type Surface struct {
	Canonical        string
	CanonicalSHA256  string
	SortedSHA256     string
	WireSHA256       string
	Tools            []Tool
	Excluded         []string
	Flags            []string
	SchemaInvalid    bool
	SchemaInvalidWhy string
}

// ErrSchemaInvalid reports a surface that cannot be resolved under
// methodology 2 and therefore fails as schema_invalid.
var ErrSchemaInvalid = errors.New("surface could not be resolved under the $ref policy")

// WireHash is the SHA-256 over the raw tools/list response bytes as received,
// concatenated across pages in order (methodology 8).
func WireHash(pages [][]byte) string {
	h := sha256.New()
	for _, p := range pages {
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Canonicalize builds the canonical serialization of a tool surface. rawTools
// must be the tool objects exactly as received, in wire order.
func Canonicalize(rawTools [][]byte, pages [][]byte, policy RefPolicy) (*Surface, error) {
	s := &Surface{WireSHA256: WireHash(pages)}
	kept := &Value{Kind: KindArray}
	flagSet := map[string]bool{}

	for i, raw := range rawTools {
		parsed, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("tool %d is not valid JSON: %w", i, err)
		}
		if parsed.Kind != KindObject {
			return nil, fmt.Errorf("tool %d is not a JSON object", i)
		}

		name := ""
		if n, ok := parsed.Get("name"); ok && n.Kind == KindString {
			name = n.Str
		}
		tool := Tool{Name: name, Descriptor: descriptorText(parsed)}
		if ti, ok := parsed.Get("title"); ok && ti.Kind == KindString {
			tool.Title = ti.Str
		}
		if d, ok := parsed.Get("description"); ok && d.Kind == KindString {
			tool.Description = d.Str
		}

		norm := &Value{Kind: KindObject, Fields: map[string]*Value{}}
		var toolFlags []string
		for _, k := range parsed.Keys {
			if !allowed(k) {
				continue
			}
			v := parsed.Fields[k]
			if k == "inputSchema" || k == "outputSchema" {
				resolved, flags := ResolveRefs(v, policy)
				v = resolved
				toolFlags = append(toolFlags, flags...)
			}
			norm.Set(k, v)
		}
		tool.Flags = dedupe(toolFlags)
		for _, f := range tool.Flags {
			flagSet[f] = true
		}

		if hasFlag(tool.Flags, FlagExternalRef) || hasFlag(tool.Flags, FlagBadPointer) {
			tool.Excluded = true
			s.Excluded = append(s.Excluded, name)
			s.Tools = append(s.Tools, tool)
			continue
		}

		tool.Canonical = norm.Serialize()
		if is, ok := norm.Get("inputSchema"); ok {
			tool.InputSchemaJSON = is.Serialize()
		}
		s.Tools = append(s.Tools, tool)
		kept.Arr = append(kept.Arr, norm)
	}

	for f := range flagSet {
		s.Flags = append(s.Flags, f)
	}
	s.Flags = dedupe(s.Flags)

	if len(rawTools) > 0 && len(kept.Arr) == 0 {
		s.SchemaInvalid = true
		s.SchemaInvalidWhy = "every tool schema was excluded under the $ref policy"
		return s, ErrSchemaInvalid
	}

	s.Canonical = kept.Serialize()
	s.CanonicalSHA256 = sha256Hex(s.Canonical)
	s.SortedSHA256 = sha256Hex(kept.SerializeSorted())
	return s, nil
}

// Counted returns the tools that survived the $ref policy.
func (s *Surface) Counted() []Tool {
	var out []Tool
	for _, t := range s.Tools {
		if !t.Excluded {
			out = append(out, t)
		}
	}
	return out
}

// descriptorText builds the authoritative descriptor with the shared join in
// the retrieval package, so the string this field carries is the same string
// the retrievability index and the hygiene grade are computed over.
func descriptorText(tool *Value) string {
	str := func(k string) string {
		if v, ok := tool.Get(k); ok && v.Kind == KindString {
			return v.Str
		}
		return ""
	}
	return retrieval.Descriptor(str("name"), str("title"), str("description"))
}

func allowed(key string) bool {
	for _, k := range toolFieldAllow {
		if k == key {
			return true
		}
	}
	return false
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
