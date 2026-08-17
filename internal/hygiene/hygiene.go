// Package hygiene computes the schema hygiene grade of methodology 6: six
// mechanical dimensions over an already-enumerated tool surface, published
// individually and as a letter grade on their mean.
//
// The grade sits in the same row as the cost figure by design. A standalone
// quality score does not constrain a cost ranking; a grade beside the token
// count means an author cannot cut their published cost by deleting
// descriptions without the deletion showing up next to it.
//
// Every dimension is mechanical and therefore gameable by an author who reads
// this file. That is the trade the project accepts: a mechanical rule can be
// reproduced by a critic, and a judged one cannot.
package hygiene

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/lopster568/loadline/internal/retrieval"
)

// SimilarityThreshold is the token-Jaccard value at or above which two sibling
// descriptors are flagged as insufficiently disambiguated. It is calibrated,
// not chosen: see the methodology 0.2.0 changelog for the measured pair
// distribution on filesystem, playwright, and context7 that fixes it, and for
// the highest-scoring legitimate pair it was set to clear.
const SimilarityThreshold = 0.60

// Grade band floors, published so a letter can be recomputed from the score.
const (
	GradeAFloor = 90.0
	GradeBFloor = 75.0
	GradeCFloor = 60.0
	GradeDFloor = 40.0
)

// Description length band from methodology 6 dimension 1.
const (
	MinDescriptionChars = 20
	MaxDescriptionChars = 1000
)

// sentenceMinWords is the word count below which a description is treated as a
// label rather than a sentence.
const sentenceMinWords = 4

// maxSchemaDepth bounds the input-schema walk. The $ref policy of methodology 2
// already leaves recursive references unresolved, so this is a belt-and-braces
// bound rather than a cycle guard.
const maxSchemaDepth = 12

// KindMeasured labels the block as harness output.
const KindMeasured = "measured"

// Tool is the input surface for one tool.
type Tool struct {
	Name        string
	Title       string
	Description string
	// Descriptor is the canonicalizer's join of the three fields above. It is
	// the token base of dimension 6, carried through from canon.Tool rather
	// than rejoined here so that dimension 6 and the retrievability index of
	// methodology 5 read the same string. A caller holding no canonical
	// surface may leave it empty, in which case the shared join is used.
	Descriptor string
	// InputSchemaJSON is the tool's inputSchema after $ref inlining, as the
	// canonicalizer produced it. An empty string means the tool declared none.
	InputSchemaJSON string
}

// descriptor returns the canonicalizer's descriptor when one was supplied and
// the shared join otherwise.
func (t Tool) descriptor() string {
	return retrieval.Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		Descriptor:  t.Descriptor,
	}.DescriptorText()
}

// Dimensions are the six sub-scores, each on 0 to 100. All six are published,
// because a single letter hides which dimension is failing.
type Dimensions struct {
	DescriptionAdequacy   float64 `json:"description_adequacy"`
	WhenToUseSignal       float64 `json:"when_to_use_signal"`
	ParameterDescriptions float64 `json:"parameter_descriptions"`
	EnumDocumentation     float64 `json:"enum_documentation"`
	NamingClarity         float64 `json:"naming_clarity"`
	Disambiguation        float64 `json:"disambiguation"`
}

// Pair is one flagged sibling pair, published as the evidence for a
// disambiguation score below 100.
type Pair struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Similarity float64 `json:"similarity"`
}

// Grade is the published per-server hygiene block.
type Grade struct {
	Grade      string     `json:"grade"`
	Score      float64    `json:"score"`
	Dimensions Dimensions `json:"dimensions"`
	// SimilarityThreshold is stamped on the row so a reader can recompute
	// dimension 6 without reading the harness source.
	SimilarityThreshold float64 `json:"similarity_threshold"`
	SimilarPairs        []Pair  `json:"similar_pairs,omitempty"`
	Kind                string  `json:"kind"`
}

// Compute grades a surface. It contacts nothing: the input is the surface the
// sweep already enumerated.
//
// A surface with no tools returns nil, which the caller publishes as
// "hygiene": null under the contract of methodology 6. Grading nothing produced
// a mix of vacuous 100s and vacuous 0s that averaged to 50 and printed as a
// grade of D, which reads as a measured judgement on a server that put no tools
// in front of a model. There is nothing to grade, and the honest publication of
// nothing is an absence, not a D.
func Compute(tools []Tool) *Grade {
	if len(tools) == 0 {
		return nil
	}
	pairs := similarPairs(tools, SimilarityThreshold)
	d := Dimensions{
		DescriptionAdequacy:   descriptionAdequacy(tools),
		WhenToUseSignal:       whenToUseSignal(tools),
		ParameterDescriptions: parameterDescriptions(tools),
		EnumDocumentation:     enumDocumentation(tools),
		NamingClarity:         namingClarity(tools),
		Disambiguation:        disambiguation(tools, pairs),
	}
	score := round2((d.DescriptionAdequacy + d.WhenToUseSignal + d.ParameterDescriptions +
		d.EnumDocumentation + d.NamingClarity + d.Disambiguation) / 6)
	return &Grade{
		Grade:               letter(score),
		Score:               score,
		Dimensions:          d,
		SimilarityThreshold: SimilarityThreshold,
		SimilarPairs:        pairs,
		Kind:                KindMeasured,
	}
}

// letter maps a mean score onto a band. The six dimensions are averaged
// unweighted: weighting them would assert a calibration against a usability
// outcome that this project has not run, and known limitation 8 says so.
func letter(score float64) string {
	switch {
	case score >= GradeAFloor:
		return "A"
	case score >= GradeBFloor:
		return "B"
	case score >= GradeCFloor:
		return "C"
	case score >= GradeDFloor:
		return "D"
	}
	return "F"
}

// descriptionAdequacy is dimension 1: a description exists, sits in the length
// band, and reads as a sentence rather than a label. Each tool scores the mean
// of the three checks, so a present but stunted description scores above a
// missing one and below a written one.
func descriptionAdequacy(tools []Tool) float64 {
	if len(tools) == 0 {
		return 0
	}
	var sum float64
	for _, t := range tools {
		desc := strings.TrimSpace(t.Description)
		checks := 0
		if desc != "" {
			checks++
		}
		if n := len([]rune(desc)); n >= MinDescriptionChars && n <= MaxDescriptionChars {
			checks++
		}
		if sentenceLike(desc) {
			checks++
		}
		sum += float64(checks) / 3
	}
	return pct(sum / float64(len(tools)))
}

// sentenceLike is the mechanical stand-in for prose: at least four
// whitespace-separated words, opening on a letter. It accepts a fragment that
// happens to be four words long and rejects a terse but complete three-word
// instruction, which is the accuracy a rule this cheap buys.
func sentenceLike(desc string) bool {
	fields := strings.Fields(desc)
	if len(fields) < sentenceMinWords {
		return false
	}
	first := []rune(fields[0])
	return len(first) > 0 && unicode.IsLetter(first[0])
}

// whenToUseKeywords detect a stated selection boundary: text telling a model
// when to reach for this tool rather than a sibling.
var whenToUseKeywords = []string{
	"use this when", "use when", "use this tool when", "use this for",
	"use it when", "useful when", "useful for", "call this when",
	"call this tool", "when you need", "when the user", "if you need",
	"only use", "do not use", "don't use", "avoid using", "instead of",
	"prefer this", "rather than", "unlike", "for cases where", "in cases where",
	"should be used", "this tool is for",
}

// whenToUseSignal is dimension 2. Methodology 6 flags this as the weakest of
// the six and that has not changed: keyword detection cannot tell a real
// selection boundary from the phrase "use this for" pasted into every
// description, and an author who reads this list can satisfy it in one edit.
// It is kept because its failure direction is safe. A server that states no
// boundary anywhere reliably scores zero here, which is a true finding; a
// server that games it buys one dimension of six and leaves the other five
// unmoved.
func whenToUseSignal(tools []Tool) float64 {
	if len(tools) == 0 {
		return 0
	}
	hits := 0
	for _, t := range tools {
		lower := strings.ToLower(t.Description)
		for _, kw := range whenToUseKeywords {
			if strings.Contains(lower, kw) {
				hits++
				break
			}
		}
	}
	return pct(float64(hits) / float64(len(tools)))
}

// property is one input-schema property found by the walk.
type property struct {
	described bool
	enum      []string
	enumDesc  string
}

// parameterDescriptions is dimension 3: the fraction of input-schema properties
// carrying a non-empty description, counted at every depth rather than only at
// the top level, since a nested object's fields cost the same context and are
// read the same way. A surface that declares no properties at all scores 100:
// there is nothing undocumented, and penalising a parameterless surface would
// reward adding parameters.
func parameterDescriptions(tools []Tool) float64 {
	total, described := 0, 0
	for _, t := range tools {
		for _, p := range walkTool(t) {
			total++
			if p.described {
				described++
			}
		}
	}
	if total == 0 {
		return 100
	}
	return pct(float64(described) / float64(total))
}

// enumDocumentation is dimension 4. An enum is documented when its own
// description names at least one of the allowed values, or when every allowed
// value is self-evident on its face. A surface with no enums scores 100.
func enumDocumentation(tools []Tool) float64 {
	total, documented := 0, 0
	for _, t := range tools {
		for _, p := range walkTool(t) {
			if len(p.enum) == 0 {
				continue
			}
			total++
			if enumIsDocumented(p) {
				documented++
			}
		}
	}
	if total == 0 {
		return 100
	}
	return pct(float64(documented) / float64(total))
}

// enumIsDocumented reports whether the property's own description names at
// least one allowed value, falling back to whether every value is self-evident.
//
// Naming is a whole-token match, not a substring one. A substring test lets a
// two-letter value pass on any description that happens to contain those
// letters: "ro" is inside "zero", "in" is inside "within", "id" is inside
// "valid", so the shortest and most cryptic enums, the ones the dimension exists
// to catch, were the easiest to score as documented by accident.
func enumIsDocumented(p property) bool {
	if desc := strings.TrimSpace(p.enumDesc); desc != "" {
		named := map[string]bool{}
		for _, tok := range enumTokens(desc) {
			named[tok] = true
		}
		for _, v := range p.enum {
			toks := enumTokens(v)
			if len(toks) == 0 {
				// A value with no token at all, such as an empty string, cannot
				// be named; treating it as named would pass on every
				// description.
				continue
			}
			all := true
			for _, tok := range toks {
				if !named[tok] {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return allSelfEvident(p.enum)
}

// enumTokens is the token basis for dimension 4: the shared splitter, keeping
// short fragments, lowercased. Short fragments are kept here although the
// retrieval tokenizer drops them, because "1" and "x" are values a description
// can legitimately name even though they carry no retrieval signal. Stopwords
// are not removed for the same reason: the stopword list belongs to query
// derivation, and an enum value of "in" or "not" is still a value.
func enumTokens(s string) []string {
	pieces := retrieval.SplitIdentifier(s, retrieval.SplitOpts{KeepShort: true})
	out := make([]string, 0, len(pieces))
	for _, piece := range pieces {
		out = append(out, strings.ToLower(piece))
	}
	return out
}

// allSelfEvident reports whether every allowed value is a readable word rather
// than a code: three or more characters, opening on a letter, and made only of
// letters, digits, and the separators an identifier uses. "read" and
// "read_only" pass; "rw", "1", and "x7" do not.
func allSelfEvident(values []string) bool {
	for _, v := range values {
		runes := []rune(v)
		if len(runes) < 3 || !unicode.IsLetter(runes[0]) {
			return false
		}
		for _, r := range runes {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
				continue
			}
			return false
		}
	}
	return len(values) > 0
}

// namingClarity is dimension 5, the mean of three checks over the server's tool
// names: conformance to the specification's name rule (1 to 128 characters, an
// identifier charset, unique within the server), consistency of casing style
// across the surface, and freedom from single-letter or bare-numeric fragments.
func namingClarity(tools []Tool) float64 {
	if len(tools) == 0 {
		return 0
	}
	seen := map[string]int{}
	for _, t := range tools {
		seen[t.Name]++
	}

	conforming, clean := 0, 0
	styles := map[string]int{}
	for _, t := range tools {
		if conformsToSpec(t.Name) && seen[t.Name] == 1 {
			conforming++
		}
		if !hasWeakFragment(t.Name) {
			clean++
		}
		styles[styleOf(t.Name)]++
	}
	dominant := 0
	for _, n := range styles {
		if n > dominant {
			dominant = n
		}
	}
	n := float64(len(tools))
	mean := (float64(conforming)/n + float64(dominant)/n + float64(clean)/n) / 3
	return pct(mean)
}

// conformsToSpec applies the name rule methodology 6 dimension 5 cites: 1 to
// 128 characters drawn from [a-zA-Z0-9_-]. The charset is ASCII and closed, so
// a dotted name like "jira.search" and any non-ASCII letter fail here even
// though Go's unicode tables would accept them. The rule the grade publishes
// and the rule the grade applies have to be the same rule.
func conformsToSpec(name string) bool {
	runes := []rune(name)
	if len(runes) < 1 || len(runes) > 128 {
		return false
	}
	for _, r := range runes {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// hasWeakFragment flags a name whose fragments include a single letter or a
// bare number: read_f, tool_2, browser1. Both are names that tell a model
// nothing about which sibling it is choosing.
func hasWeakFragment(name string) bool {
	fragments := splitName(name)
	if len(fragments) == 0 {
		return true
	}
	for _, f := range fragments {
		if len([]rune(f)) < 2 {
			return true
		}
		numeric := true
		for _, r := range f {
			if !unicode.IsDigit(r) {
				numeric = false
				break
			}
		}
		if numeric {
			return true
		}
	}
	return false
}

// splitName breaks a tool name into fragments with the shared splitter,
// keeping single-character fragments that the retrieval tokenizer discards and
// splitting digit runs off, because a single-letter or bare-numeric fragment is
// precisely what dimension 5 looks for.
func splitName(name string) []string {
	return retrieval.SplitIdentifier(name, retrieval.SplitOpts{DigitRuns: true, KeepShort: true})
}

// styleOf classifies a name's casing convention. Consistency matters because a
// surface mixing conventions makes a model guess at the name it has not seen.
func styleOf(name string) string {
	switch {
	case strings.Contains(name, "_"):
		return "snake"
	case strings.Contains(name, "-"):
		return "kebab"
	case strings.Contains(name, "."):
		return "dotted"
	case strings.ToLower(name) != name:
		return "camel"
	}
	return "flat"
}

// disambiguation is dimension 6, scored as the fraction of tools not involved
// in any over-threshold pair. Scoring on pairs instead would make the dimension
// quadratic in surface size, so one bad pair would cost a 24-tool server far
// less than it costs a 4-tool one for the same defect. A surface with fewer
// than two tools scores 100: there is no sibling to be confused with.
func disambiguation(tools []Tool, pairs []Pair) float64 {
	if len(tools) < 2 {
		return 100
	}
	flagged := map[string]bool{}
	for _, p := range pairs {
		flagged[p.A] = true
		flagged[p.B] = true
	}
	return pct(1 - float64(len(flagged))/float64(len(tools)))
}

// similarPairs returns every sibling pair at or above the threshold, by token
// Jaccard over name, title, and description with stopwords removed. Stopwords
// are dropped because two unrelated tools share "the", "a", and "of", and
// leaving them in would raise every pair's similarity by a constant that has
// nothing to do with whether the tools are confusable.
func similarPairs(tools []Tool, threshold float64) []Pair {
	sets := make([]map[string]bool, len(tools))
	for i, t := range tools {
		sets[i] = tokenSet(t)
	}
	var out []Pair
	for i := 0; i < len(tools); i++ {
		for j := i + 1; j < len(tools); j++ {
			s := jaccard(sets[i], sets[j])
			if s >= threshold {
				out = append(out, Pair{A: tools[i].Name, B: tools[j].Name, Similarity: retrieval.Round4(s)})
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Similarity != out[b].Similarity {
			return out[a].Similarity > out[b].Similarity
		}
		if out[a].A != out[b].A {
			return out[a].A < out[b].A
		}
		return out[a].B < out[b].B
	})
	return out
}

func tokenSet(t Tool) map[string]bool {
	set := map[string]bool{}
	for _, w := range retrieval.ContentWords(t.descriptor()) {
		set[w] = true
	}
	return set
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// walkTool collects every property of a tool's input schema, at every depth.
func walkTool(t Tool) []property {
	if strings.TrimSpace(t.InputSchemaJSON) == "" {
		return nil
	}
	var node map[string]json.RawMessage
	if err := json.Unmarshal([]byte(t.InputSchemaJSON), &node); err != nil {
		return nil
	}
	var out []property
	walkSchema(node, 0, &out)
	return out
}

func walkSchema(node map[string]json.RawMessage, depth int, out *[]property) {
	if node == nil || depth > maxSchemaDepth {
		return
	}
	if raw, ok := node["properties"]; ok {
		var props map[string]json.RawMessage
		if json.Unmarshal(raw, &props) == nil {
			// Property order in a JSON object is not preserved by the decoder,
			// and it does not need to be here: every dimension over properties
			// is a fraction, and a fraction is order-independent.
			names := make([]string, 0, len(props))
			for name := range props {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				var child map[string]json.RawMessage
				if json.Unmarshal(props[name], &child) != nil {
					continue
				}
				*out = append(*out, propertyOf(child))
				walkSchema(child, depth+1, out)
			}
		}
	}
	// "items" carries two shapes. Draft 2020-12 and the MCP examples use the
	// object form, but draft-07 also allows the tuple form, an array of
	// schemas positional to the array's elements, and a schema written that way
	// is still full of properties and enums that cost context. Both forms are
	// walked; "additionalProperties" and "not" only ever take the object form,
	// and a boolean "additionalProperties" simply fails both decodes.
	for _, key := range []string{"items", "additionalProperties", "not"} {
		raw, ok := node[key]
		if !ok {
			continue
		}
		var child map[string]json.RawMessage
		if json.Unmarshal(raw, &child) == nil {
			walkSchema(child, depth+1, out)
			continue
		}
		var tuple []json.RawMessage
		if json.Unmarshal(raw, &tuple) != nil {
			continue
		}
		for _, item := range tuple {
			var element map[string]json.RawMessage
			if json.Unmarshal(item, &element) == nil {
				walkSchema(element, depth+1, out)
			}
		}
	}
	for _, key := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
		raw, ok := node[key]
		if !ok {
			continue
		}
		var branches []json.RawMessage
		if json.Unmarshal(raw, &branches) != nil {
			continue
		}
		for _, b := range branches {
			var child map[string]json.RawMessage
			if json.Unmarshal(b, &child) == nil {
				walkSchema(child, depth+1, out)
			}
		}
	}
	if raw, ok := node["$defs"]; ok {
		var defs map[string]json.RawMessage
		if json.Unmarshal(raw, &defs) == nil {
			names := make([]string, 0, len(defs))
			for name := range defs {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				var child map[string]json.RawMessage
				if json.Unmarshal(defs[name], &child) == nil {
					walkSchema(child, depth+1, out)
				}
			}
		}
	}
}

// propertyOf reads one property node. An enum is read from the property itself
// or from a single-branch wrapper the property may sit behind.
func propertyOf(node map[string]json.RawMessage) property {
	p := property{}
	if raw, ok := node["description"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			p.described = true
			p.enumDesc = s
		}
	}
	if raw, ok := node["enum"]; ok {
		var values []any
		if json.Unmarshal(raw, &values) == nil {
			for _, v := range values {
				if s, isStr := v.(string); isStr {
					p.enum = append(p.enum, s)
					continue
				}
				b, err := json.Marshal(v)
				if err == nil {
					p.enum = append(p.enum, string(b))
				}
			}
		}
	}
	return p
}

func pct(fraction float64) float64 { return round2(fraction * 100) }

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
