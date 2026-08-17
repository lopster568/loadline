package canon

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func tools(raws ...string) [][]byte {
	out := make([][]byte, len(raws))
	for i, r := range raws {
		out[i] = []byte(r)
	}
	return out
}

func mustCanon(t *testing.T, raws ...string) *Surface {
	t.Helper()
	s, err := Canonicalize(tools(raws...), tools(raws...), DefaultRefPolicy())
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	return s
}

func TestCanonicalPreservesWireKeyOrder(t *testing.T) {
	s := mustCanon(t, `{"name":"a","description":"d","inputSchema":{"type":"object"}}`)
	want := `[{"name":"a","description":"d","inputSchema":{"type":"object"}}]`
	if s.Canonical != want {
		t.Errorf("canonical = %s, want %s", s.Canonical, want)
	}
}

func TestCanonicalIsDeterministic(t *testing.T) {
	raw := `{"name":"a","inputSchema":{"type":"object","properties":{"z":{"type":"string"},"a":{"type":"number"}}}}`
	first := mustCanon(t, raw)
	second := mustCanon(t, raw)
	if first.CanonicalSHA256 != second.CanonicalSHA256 {
		t.Error("canonical hash is not stable across runs")
	}
	if first.SortedSHA256 != second.SortedSHA256 {
		t.Error("sorted hash is not stable across runs")
	}
}

// Methodology 1.5 fixes the canonical string at the server's own key order, so
// a reordering server changes canonical_sha256. Methodology 8 also wants a
// digest that tolerates cosmetic reordering, which is what the sorted digest
// provides.
func TestKeyOrderChangesCanonicalHashButNotSortedHash(t *testing.T) {
	a := mustCanon(t, `{"name":"a","description":"d","inputSchema":{"type":"object"}}`)
	b := mustCanon(t, `{"inputSchema":{"type":"object"},"description":"d","name":"a"}`)

	if a.CanonicalSHA256 == b.CanonicalSHA256 {
		t.Error("canonical hash ignored key order, but methodology 1.5 keeps wire order")
	}
	if a.SortedSHA256 != b.SortedSHA256 {
		t.Errorf("sorted hash differs across key order: %s vs %s", a.SortedSHA256, b.SortedSHA256)
	}
}

func TestToolOrderChangesWireHashNotSortedHash(t *testing.T) {
	one := `{"name":"a","description":"first"}`
	two := `{"name":"b","description":"second"}`
	fwd := mustCanon(t, one, two)
	rev := mustCanon(t, two, one)

	if fwd.WireSHA256 == rev.WireSHA256 {
		t.Error("wire hash must detect tool ordering changes")
	}
	if fwd.SortedSHA256 != rev.SortedSHA256 {
		t.Error("sorted hash must tolerate tool ordering changes")
	}
}

// The name sort exists to tolerate a cosmetic reorder of the tools array. A
// nested array of named objects inside an inputSchema is not that: reordering
// it is a real surface change, and the sorted digest has to catch it or it
// reports "nothing moved" about a surface that did.
func TestNestedNamedArrayReorderChangesTheSortedHash(t *testing.T) {
	const shape = `{"name":"a","inputSchema":{"type":"object","properties":{"filters":{"type":"array","prefixItems":[%s,%s]}}}}`
	first := `{"name":"alpha","const":1}`
	second := `{"name":"beta","const":2}`
	fwd := mustCanon(t, fmt.Sprintf(shape, first, second))
	rev := mustCanon(t, fmt.Sprintf(shape, second, first))

	if fwd.CanonicalSHA256 == rev.CanonicalSHA256 {
		t.Fatal("the order-preserving hash missed a nested reorder")
	}
	if fwd.SortedSHA256 == rev.SortedSHA256 {
		t.Error("the sorted hash tolerated a reorder inside inputSchema; only the tools array may be name-sorted")
	}

	// The tolerance methodology 8 does ask for still holds with such an array
	// present: reordering the tools themselves must not move the sorted hash.
	toolA := fmt.Sprintf(shape, first, second)
	toolB := strings.Replace(toolA, `"name":"a"`, `"name":"z"`, 1)
	if mustCanon(t, toolA, toolB).SortedSHA256 != mustCanon(t, toolB, toolA).SortedSHA256 {
		t.Error("sorted hash stopped tolerating a tools-array reorder")
	}
}

func TestFieldAllowlist(t *testing.T) {
	s := mustCanon(t, `{"name":"a","description":"d","serverSpecific":"drop me","annotations":{"x-mcp-header":"keep"},"icons":[{"src":"i"}],"_meta":{"k":1}}`)
	for _, want := range []string{`"annotations"`, `"x-mcp-header"`, `"icons"`, `"_meta"`} {
		if !strings.Contains(s.Canonical, want) {
			t.Errorf("canonical dropped %s: %s", want, s.Canonical)
		}
	}
	if strings.Contains(s.Canonical, "serverSpecific") {
		t.Errorf("canonical kept a non-allowlisted field: %s", s.Canonical)
	}
}

func TestInternalRefInlinedAndDefsDropped(t *testing.T) {
	raw := `{"name":"a","inputSchema":{"type":"object","$defs":{"Id":{"type":"string","description":"an id"}},` +
		`"properties":{"one":{"$ref":"#/$defs/Id"},"two":{"$ref":"#/$defs/Id"}}}}`
	s := mustCanon(t, raw)

	if strings.Contains(s.Canonical, "$ref") {
		t.Errorf("internal $ref survived inlining: %s", s.Canonical)
	}
	if strings.Contains(s.Canonical, "$defs") {
		t.Errorf("$defs was not dropped after inlining: %s", s.Canonical)
	}
	if n := strings.Count(s.Canonical, "an id"); n != 2 {
		t.Errorf("inlined description appears %d times, want 2", n)
	}
	if len(s.Tools[0].Flags) != 0 {
		t.Errorf("clean inlining raised flags %v", s.Tools[0].Flags)
	}
}

func TestRefSiblingKeysWin(t *testing.T) {
	raw := `{"name":"a","inputSchema":{"$defs":{"B":{"type":"string","description":"base"}},` +
		`"properties":{"p":{"$ref":"#/$defs/B","description":"override"}}}}`
	s := mustCanon(t, raw)
	if !strings.Contains(s.Canonical, `"description":"override"`) {
		t.Errorf("sibling keyword did not win over the inlined target: %s", s.Canonical)
	}
	if strings.Contains(s.Canonical, "base") {
		t.Errorf("inlined target overwrote the sibling: %s", s.Canonical)
	}
}

func TestRecursiveRefLeftUnresolvedAndFlagged(t *testing.T) {
	raw := `{"name":"a","inputSchema":{"$defs":{"Node":{"type":"object","properties":{"child":{"$ref":"#/$defs/Node"}}}},` +
		`"properties":{"root":{"$ref":"#/$defs/Node"}}}}`
	s := mustCanon(t, raw)

	tool := s.Tools[0]
	if !hasFlag(tool.Flags, FlagRecursiveRef) {
		t.Errorf("recursive $ref not flagged: %v", tool.Flags)
	}
	if tool.Excluded {
		t.Error("a recursive $ref should flag the tool, not exclude it")
	}
	if !strings.Contains(tool.Canonical, "$defs") {
		t.Error("$defs must be retained when an unresolved recursive $ref remains")
	}
}

func TestExternalRefExcludesTool(t *testing.T) {
	s := mustCanon(t,
		`{"name":"ok","inputSchema":{"type":"object"}}`,
		`{"name":"bad","inputSchema":{"properties":{"p":{"$ref":"https://example.com/schema.json"}}}}`,
	)
	if s.Tools[1].Excluded != true {
		t.Error("tool with a network $ref was counted")
	}
	if !hasFlag(s.Tools[1].Flags, FlagExternalRef) {
		t.Errorf("flags = %v", s.Tools[1].Flags)
	}
	if strings.Contains(s.Canonical, "example.com") {
		t.Error("excluded tool leaked into the canonical serialization")
	}
	if len(s.Counted()) != 1 {
		t.Errorf("counted %d tools, want 1", len(s.Counted()))
	}
}

func TestWholeSurfaceExternalRefIsSchemaInvalid(t *testing.T) {
	_, err := Canonicalize(
		tools(`{"name":"bad","inputSchema":{"$ref":"http://example.com/s.json"}}`),
		tools(`{"name":"bad"}`),
		DefaultRefPolicy(),
	)
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("err = %v, want ErrSchemaInvalid", err)
	}
}

func TestRefDepthBoundFlags(t *testing.T) {
	policy := RefPolicy{MaxDepth: 2, MaxSubschemas: 100, Budget: time.Second}
	s, err := Canonicalize(
		tools(`{"name":"a","inputSchema":{"type":"object","properties":{"x":{"properties":{"y":{"properties":{"z":{"type":"string"}}}}}}}}`),
		tools(`{"name":"a"}`),
		policy,
	)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if !hasFlag(s.Tools[0].Flags, FlagBoundHit) {
		t.Errorf("depth bound not flagged: %v", s.Tools[0].Flags)
	}
}

func TestUnresolvableInternalPointerExcludesTool(t *testing.T) {
	s := mustCanon(t,
		`{"name":"ok","inputSchema":{"type":"object"}}`,
		`{"name":"dangling","inputSchema":{"properties":{"p":{"$ref":"#/$defs/Missing"}}}}`,
	)
	if !s.Tools[1].Excluded {
		t.Error("dangling internal pointer was counted")
	}
	if !hasFlag(s.Tools[1].Flags, FlagBadPointer) {
		t.Errorf("flags = %v", s.Tools[1].Flags)
	}
}

func TestPerToolCanonicalAndInputSchema(t *testing.T) {
	s := mustCanon(t, `{"name":"a","description":"d","inputSchema":{"type":"object","properties":{}}}`)
	tool := s.Tools[0]
	if tool.Canonical != `{"name":"a","description":"d","inputSchema":{"type":"object","properties":{}}}` {
		t.Errorf("per-tool canonical = %s", tool.Canonical)
	}
	if tool.InputSchemaJSON != `{"type":"object","properties":{}}` {
		t.Errorf("input schema = %s", tool.InputSchemaJSON)
	}
	if tool.Descriptor != "a d" {
		t.Errorf("descriptor = %q", tool.Descriptor)
	}
}

func TestNumbersAndUnicodeSurviveRoundTrip(t *testing.T) {
	s := mustCanon(t, `{"name":"a","inputSchema":{"maximum":1.50,"minimum":-0,"title":"café <b>"}}`)
	if !strings.Contains(s.Canonical, "1.50") {
		t.Errorf("number literal was reformatted: %s", s.Canonical)
	}
	if !strings.Contains(s.Canonical, "café <b>") {
		t.Errorf("string was HTML-escaped or mangled: %s", s.Canonical)
	}
}

func TestEmptySurface(t *testing.T) {
	s, err := Canonicalize(nil, tools(`{"tools":[]}`), DefaultRefPolicy())
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if s.Canonical != "[]" {
		t.Errorf("canonical = %q, want []", s.Canonical)
	}
	if s.WireSHA256 == "" {
		t.Error("wire hash must be recorded even for an empty surface")
	}
}
