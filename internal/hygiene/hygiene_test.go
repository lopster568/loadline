package hygiene

import (
	"strings"
	"testing"
	"time"

	"github.com/lopster568/loadline/internal/canon"
	"github.com/lopster568/loadline/internal/retrieval"
)

// Methodology 5 indexes the canonicalizer's descriptor and dimension 6 of
// methodology 6 tokenizes it. Those are the same string or the two published
// metrics are reading different documents, so this asserts the whole chain:
// canon builds one descriptor, both metric packages consume that field rather
// than rejoining the parts, and the fallback join agrees with it.
func TestRetrievalAndHygieneShareOneDescriptor(t *testing.T) {
	raw := []byte(`{"name":"read_text_file","title":"Read Text File",` +
		`"description":"Read the complete contents of a file as text.",` +
		`"inputSchema":{"type":"object","properties":{}}}`)
	surface, err := canon.Canonicalize([][]byte{raw}, [][]byte{raw},
		canon.RefPolicy{MaxDepth: 8, MaxSubschemas: 100, Budget: time.Second})
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	ct := surface.Counted()[0]
	if ct.Descriptor == "" {
		t.Fatal("the canonicalizer published an empty descriptor")
	}

	// Projected exactly as the sweep projects them.
	rt := retrieval.Tool{Name: ct.Name, Title: ct.Title, Description: ct.Description, Descriptor: ct.Descriptor}
	ht := Tool{Name: ct.Name, Title: ct.Title, Description: ct.Description, Descriptor: ct.Descriptor,
		InputSchemaJSON: ct.InputSchemaJSON}

	if rt.DescriptorText() != ct.Descriptor {
		t.Errorf("retrieval document text %q is not the canonical descriptor %q", rt.DescriptorText(), ct.Descriptor)
	}
	if ht.descriptor() != ct.Descriptor {
		t.Errorf("hygiene token base %q is not the canonical descriptor %q", ht.descriptor(), ct.Descriptor)
	}

	// The fallback used by a caller holding no canonical surface is the same
	// join, not a second one.
	bare := Tool{Name: ct.Name, Title: ct.Title, Description: ct.Description}
	if bare.descriptor() != ct.Descriptor {
		t.Errorf("fallback join %q diverged from the canonical descriptor %q", bare.descriptor(), ct.Descriptor)
	}
}

func describedTool(name, desc string) Tool {
	return Tool{Name: name, Description: desc, InputSchemaJSON: `{"type":"object","properties":{}}`}
}

// Dimension 6 is the reason the grade cannot be gamed by shipping the same tool
// twice under two names. A near-duplicate pair must flag; a pair that merely
// shares a domain must not.
func TestDisambiguationFlagsNearDuplicatesOnly(t *testing.T) {
	nearDuplicate := []Tool{
		describedTool("list_directory",
			"Get a detailed listing of all files and directories in a specified path. Results distinguish between files and directories with prefixes. Only works within allowed directories."),
		describedTool("list_directory_entries",
			"Get a detailed listing of all files and directories in a specified path. Results distinguish between files and directories with prefixes. Only works within allowed directories."),
	}
	pairs := similarPairs(nearDuplicate, SimilarityThreshold)
	if len(pairs) != 1 {
		t.Fatalf("near-identical pair did not flag: %+v", pairs)
	}
	if pairs[0].Similarity < SimilarityThreshold {
		t.Errorf("flagged pair below threshold: %+v", pairs[0])
	}
	if got := disambiguation(nearDuplicate, pairs); got != 0 {
		t.Errorf("disambiguation = %v, want 0 when both tools are flagged", got)
	}

	distinct := []Tool{
		describedTool("read_file",
			"Read the complete contents of a file from the filesystem and return it as text. Only works within allowed directories."),
		describedTool("create_directory",
			"Create a new directory, including any missing parent directories along the path. Only works within allowed directories."),
	}
	if pairs := similarPairs(distinct, SimilarityThreshold); len(pairs) != 0 {
		t.Errorf("distinct pair false-positived: %+v", pairs)
	}
	if got := disambiguation(distinct, nil); got != 100 {
		t.Errorf("disambiguation = %v, want 100", got)
	}
}

// The calibration recorded in the methodology 0.2.0 changelog rests on two
// facts about the measured corpus: the highest pair judged legitimate scored
// 0.4211 and the one flagged pair scored 0.8519. The threshold has to sit
// between them with room on both sides, or the calibration note is wrong.
func TestSimilarityThresholdSitsInTheCalibratedGap(t *testing.T) {
	const highestLegitimate = 0.4211
	const flaggedPair = 0.8519
	if SimilarityThreshold <= highestLegitimate {
		t.Errorf("threshold %v would flag the legitimate pair at %v", SimilarityThreshold, highestLegitimate)
	}
	if SimilarityThreshold > flaggedPair {
		t.Errorf("threshold %v would clear the near-duplicate pair at %v", SimilarityThreshold, flaggedPair)
	}
}

func TestDisambiguationScoresTheFractionOfCleanTools(t *testing.T) {
	tools := make([]Tool, 0, 4)
	tools = append(tools,
		describedTool("alpha", "Rotate the signing credentials used by a deployment target."),
		describedTool("beta", "Rotate the signing credentials used by a deployment target."),
		describedTool("gamma", "List every open pull request in a source repository."),
		describedTool("delta", "Upload a build artifact to the release bucket."),
	)
	pairs := similarPairs(tools, SimilarityThreshold)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %+v", pairs)
	}
	if got := disambiguation(tools, pairs); got != 50 {
		t.Errorf("disambiguation = %v, want 50: two of four tools are flagged", got)
	}
}

func TestSingleToolSurfaceHasNothingToDisambiguate(t *testing.T) {
	tools := []Tool{describedTool("only", "Do the one thing this server does, carefully.")}
	if got := disambiguation(tools, nil); got != 100 {
		t.Errorf("disambiguation = %v, want 100 on a one-tool surface", got)
	}
}

func TestDescriptionAdequacyBands(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want float64
	}{
		{"missing", "", 0},
		{"label_only", "read", 33.33},                  // present only: too short, and not a sentence
		{"short_sentence", "Read a file now", 66.67},   // present and sentence-like, under 20 chars
		{"good", "Read the contents of a file.", 100},  // all three checks
		{"too_long", "Read. " + longText(1200), 66.67}, // present and sentence-like, over the band
	}
	for _, tc := range cases {
		got := descriptionAdequacy([]Tool{{Name: "t", Description: tc.desc}})
		if got != tc.want {
			t.Errorf("%s: description_adequacy = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestWhenToUseSignal(t *testing.T) {
	tools := []Tool{
		{Name: "a", Description: "Read a file. Use this when the caller already knows the path."},
		{Name: "b", Description: "Read a file."},
	}
	if got := whenToUseSignal(tools); got != 50 {
		t.Errorf("when_to_use_signal = %v, want 50", got)
	}
}

// A surface that declares no parameters has nothing undocumented. Scoring it
// zero would reward a server for adding parameters it does not need.
func TestParameterDescriptionsWithNoParameters(t *testing.T) {
	tools := []Tool{
		{Name: "a", Description: "Close the page.", InputSchemaJSON: `{"type":"object","properties":{}}`},
		{Name: "b", Description: "Open the page.", InputSchemaJSON: ""},
	}
	if got := parameterDescriptions(tools); got != 100 {
		t.Errorf("parameter_descriptions = %v, want 100 on a parameterless surface", got)
	}
}

func TestParameterDescriptionsCountsNestedProperties(t *testing.T) {
	tools := []Tool{{
		Name: "a",
		InputSchemaJSON: `{"type":"object","properties":{
			"path":{"type":"string"},
			"options":{"type":"object","description":"Extra options","properties":{
				"depth":{"type":"number","description":"How deep to walk"},
				"follow":{"type":"boolean"}
			}}
		}}`,
	}}
	// Four properties in total: path (no), options (yes), depth (yes),
	// follow (no).
	if got := parameterDescriptions(tools); got != 50 {
		t.Errorf("parameter_descriptions = %v, want 50", got)
	}
}

func TestEnumDocumentation(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		want   float64
	}{
		{
			name:   "no_enums",
			schema: `{"type":"object","properties":{"path":{"type":"string","description":"A path"}}}`,
			want:   100,
		},
		{
			name:   "description_names_a_value",
			schema: `{"type":"object","properties":{"mode":{"type":"string","description":"Either rw or ro","enum":["rw","ro"]}}}`,
			want:   100,
		},
		{
			name:   "self_evident_values",
			schema: `{"type":"object","properties":{"mode":{"type":"string","enum":["read_only","read_write"]}}}`,
			want:   100,
		},
		{
			name:   "cryptic_and_undocumented",
			schema: `{"type":"object","properties":{"mode":{"type":"string","enum":["rw","ro"]}}}`,
			want:   0,
		},
		{
			name:   "description_ignores_the_values",
			schema: `{"type":"object","properties":{"mode":{"type":"string","description":"The access mode","enum":["rw","ro"]}}}`,
			want:   0,
		},
		{
			name:   "numeric_values_need_a_description",
			schema: `{"type":"object","properties":{"level":{"type":"number","enum":[1,2,3]}}}`,
			want:   0,
		},
		{
			name:   "nested_enum_is_found",
			schema: `{"type":"object","properties":{"opts":{"type":"object","properties":{"mode":{"type":"string","enum":["rw","ro"]}}}}}`,
			want:   0,
		},
	}
	for _, tc := range cases {
		got := enumDocumentation([]Tool{{Name: "t", InputSchemaJSON: tc.schema}})
		if got != tc.want {
			t.Errorf("%s: enum_documentation = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A substring test scores a two-letter enum as documented on any description
// that happens to contain those letters. "ro" sits inside "zero", so a
// description about zero-copy transfer used to document an access-mode enum it
// never mentions.
func TestEnumDocumentationRequiresAWholeTokenMatch(t *testing.T) {
	incidental := `{"type":"object","properties":{"mode":{"type":"string",
		"description":"Use zero-copy transfer where possible","enum":["ro","rw"]}}}`
	if got := enumDocumentation([]Tool{{Name: "t", InputSchemaJSON: incidental}}); got != 0 {
		t.Errorf("enum_documentation = %v, want 0: the description names neither value", got)
	}

	named := `{"type":"object","properties":{"mode":{"type":"string",
		"description":"Pass ro for read-only access or rw to allow writes","enum":["ro","rw"]}}}`
	if got := enumDocumentation([]Tool{{Name: "t", InputSchemaJSON: named}}); got != 100 {
		t.Errorf("enum_documentation = %v, want 100: the description names both values", got)
	}

	// The same trap on the other short values the dimension exists to catch.
	for _, tc := range []struct {
		desc  string
		value string
	}{
		{"Restrict the walk to files within the workspace", "in"},
		{"Return only records that are still valid", "id"},
	} {
		p := property{enumDesc: tc.desc, enum: []string{tc.value, "zz"}}
		if enumIsDocumented(p) {
			t.Errorf("%q scored as naming %q on a substring alone", tc.desc, tc.value)
		}
	}

	// A multi-token value is named only when every one of its tokens appears.
	if !enumIsDocumented(property{enumDesc: "Pass read_only to open without writes", enum: []string{"read_only"}}) {
		t.Error("a description naming a multi-token value scored as undocumented")
	}
	if enumIsDocumented(property{enumDesc: "The value is read from disk", enum: []string{"read_only", "rw"}}) {
		t.Error("a description carrying one fragment of a multi-token value scored as naming it")
	}
}

// draft-07 allows "items" to be an array of schemas positional to the array's
// elements. A tuple written that way carries properties and enums that cost the
// same context as any other, and dimensions 3 and 4 have to count them.
func TestWalkSchemaReadsTupleFormItems(t *testing.T) {
	tuple := `{"type":"object","properties":{
		"pair":{"type":"array","description":"A coordinate pair","items":[
			{"type":"object","properties":{
				"x":{"type":"number","description":"Horizontal offset"},
				"y":{"type":"number"}
			}},
			{"type":"object","properties":{
				"unit":{"type":"string","enum":["px","em"]}
			}}
		]}
	}}`
	props := walkTool(Tool{Name: "t", InputSchemaJSON: tuple})
	if len(props) != 4 {
		t.Fatalf("walk found %d properties, want 4 (pair, x, y, unit)", len(props))
	}
	// pair, x described; y, unit not.
	if got := parameterDescriptions([]Tool{{Name: "t", InputSchemaJSON: tuple}}); got != 50 {
		t.Errorf("parameter_descriptions = %v, want 50: two of four described", got)
	}
	if got := enumDocumentation([]Tool{{Name: "t", InputSchemaJSON: tuple}}); got != 0 {
		t.Errorf("enum_documentation = %v, want 0: the tuple's enum is cryptic and undocumented", got)
	}

	// The object form still walks, and a boolean "items" is simply skipped.
	object := `{"type":"object","properties":{"tags":{"type":"array","items":{
		"type":"object","properties":{"label":{"type":"string"}}}}}}`
	if got := parameterDescriptions([]Tool{{Name: "t", InputSchemaJSON: object}}); got != 0 {
		t.Errorf("object-form items: parameter_descriptions = %v, want 0", got)
	}
	boolForm := `{"type":"object","properties":{"tags":{"type":"array","items":true,"description":"Tags"}}}`
	if got := parameterDescriptions([]Tool{{Name: "t", InputSchemaJSON: boolForm}}); got != 100 {
		t.Errorf("boolean items: parameter_descriptions = %v, want 100", got)
	}
}

// Methodology 6 dimension 5 cites the name rule as 1 to 128 characters of
// [a-zA-Z0-9_-]. The check has to be that rule and not a looser one, or the
// published rule and the applied rule are different rules.
func TestConformsToSpecMatchesTheCitedCharset(t *testing.T) {
	cases := map[string]bool{
		"browser_take_screenshot": true,
		"resolve-library-id":      true,
		"readFile2":               true,
		"a":                       true,
		"jira.search":             false,
		"read file":               false,
		"lire_fichiér":            false,
		"read/file":               false,
		"":                        false,
		strings.Repeat("a", 128):  true,
		strings.Repeat("a", 129):  false,
	}
	for name, want := range cases {
		if got := conformsToSpec(name); got != want {
			t.Errorf("conformsToSpec(%q) = %v, want %v", name, got, want)
		}
	}

	// A dotted surface loses a third of dimension 5, which is the point: the
	// spec's charset does not include the dot.
	dotted := namingClarity([]Tool{{Name: "jira.search"}, {Name: "jira.create"}})
	if dotted >= 100 {
		t.Errorf("a dotted surface scored %v on naming clarity, want a spec-conformance penalty", dotted)
	}
}

func TestNamingClarity(t *testing.T) {
	clean := []Tool{{Name: "read_file"}, {Name: "write_file"}, {Name: "list_directory"}}
	if got := namingClarity(clean); got != 100 {
		t.Errorf("clean names scored %v, want 100", got)
	}

	weak := []Tool{{Name: "read_file"}, {Name: "tool_2"}, {Name: "f"}}
	got := namingClarity(weak)
	if got >= 100 {
		t.Errorf("single-letter and numbered names scored %v, want a penalty", got)
	}

	mixed := []Tool{{Name: "read_file"}, {Name: "writeFile"}, {Name: "list-directory"}, {Name: "movefile"}}
	if namingClarity(mixed) >= namingClarity([]Tool{{Name: "read_file"}, {Name: "write_file"}, {Name: "list_directory"}, {Name: "move_file"}}) {
		t.Error("a surface mixing four casing conventions did not score below a consistent one")
	}

	dup := []Tool{{Name: "read_file"}, {Name: "read_file"}}
	if namingClarity(dup) >= 100 {
		t.Error("duplicate tool names scored full marks on spec conformance")
	}
}

func TestHasWeakFragment(t *testing.T) {
	cases := map[string]bool{
		"read_file":     false,
		"browser_tabs":  false,
		"tool_2":        true,
		"browser1":      true,
		"f":             true,
		"read_f":        true,
		"readFile":      false,
		"http2_connect": true,
	}
	for name, want := range cases {
		if got := hasWeakFragment(name); got != want {
			t.Errorf("hasWeakFragment(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestGradeBands(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89.99, "B"}, {75, "B"}, {74.9, "C"},
		{60, "C"}, {59.9, "D"}, {40, "D"}, {39.9, "F"}, {0, "F"},
	}
	for _, tc := range cases {
		if got := letter(tc.score); got != tc.want {
			t.Errorf("letter(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestComputePublishesAllSixDimensionsAndTheThreshold(t *testing.T) {
	tools := []Tool{
		{
			Name:            "read_file",
			Description:     "Read the complete contents of a file. Use this when the caller already knows the path.",
			InputSchemaJSON: `{"type":"object","properties":{"path":{"type":"string","description":"Absolute path to read"}}}`,
		},
		{
			Name:            "create_directory",
			Description:     "Create a new directory, including any missing parents along the way.",
			InputSchemaJSON: `{"type":"object","properties":{"path":{"type":"string","description":"Absolute path to create"}}}`,
		},
	}
	g := Compute(tools)
	if g == nil {
		t.Fatal("a two-tool surface graded nil")
	}
	if g.Kind != KindMeasured {
		t.Errorf("kind = %q", g.Kind)
	}
	if g.SimilarityThreshold != SimilarityThreshold {
		t.Errorf("threshold stamp = %v", g.SimilarityThreshold)
	}
	if len(g.SimilarPairs) != 0 {
		t.Errorf("distinct tools flagged a pair: %+v", g.SimilarPairs)
	}
	d := g.Dimensions
	all := []float64{d.DescriptionAdequacy, d.WhenToUseSignal, d.ParameterDescriptions,
		d.EnumDocumentation, d.NamingClarity, d.Disambiguation}
	var sum float64
	for _, v := range all {
		if v < 0 || v > 100 {
			t.Errorf("dimension out of range: %v", v)
		}
		sum += v
	}
	if want := round2(sum / 6); g.Score != want {
		t.Errorf("score = %v, want the unweighted mean %v", g.Score, want)
	}
	if g.Grade != letter(g.Score) {
		t.Errorf("grade %q does not match score %v", g.Grade, g.Score)
	}
	if d.WhenToUseSignal != 50 {
		t.Errorf("when_to_use_signal = %v, want 50", d.WhenToUseSignal)
	}
	if d.ParameterDescriptions != 100 || d.EnumDocumentation != 100 || d.Disambiguation != 100 {
		t.Errorf("dimensions = %+v", d)
	}
}

// A surface with no tools has nothing to grade. Grading it anyway mixed vacuous
// 100s (dimensions 3, 4, 6) with vacuous 0s (dimensions 1, 2, 5) into a mean of
// 50, which published as a measured D. The null contract of methodology 6 says
// an absence publishes as an absence.
func TestComputeOnAnEmptySurface(t *testing.T) {
	if g := Compute(nil); g != nil {
		t.Errorf("empty surface graded %+v, want nil", g)
	}
	if g := Compute([]Tool{}); g != nil {
		t.Errorf("zero-length surface graded %+v, want nil", g)
	}
}

func longText(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
		if i%6 == 5 {
			out[i] = ' '
		}
	}
	return string(out)
}
