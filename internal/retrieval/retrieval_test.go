package retrieval

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// fixture is the hand-checked corpus. Token lengths are 2, 2, and 5, so the
// average document length is 3 and the length-normalisation term differs
// between the short and long documents, which is the part of BM25 an
// implementation is most likely to get wrong.
func fixture() *Index {
	return NewIndex([]Doc{
		{Name: "alpha", Text: "read file"},
		{Name: "beta", Text: "write file"},
		{Name: "gamma", Text: "read file contents disk quickly"},
	})
}

func TestBM25MatchesHandCheckedScores(t *testing.T) {
	ix := fixture()

	// idf(term) = ln(1 + (N - df + 0.5) / (df + 0.5)), N = 3.
	wantIDF := map[string]float64{
		"read": 0.47000362924573563, // df 2
		"file": 0.13353139262452257, // df 3
		"disk": 0.9808292530117263,  // df 1
	}
	for term, want := range wantIDF {
		if got := ix.IDF(term); !close(got, want) {
			t.Errorf("IDF(%q) = %.17g, want %.17g", term, got, want)
		}
	}

	cases := []struct {
		terms []string
		want  map[string]float64
	}{
		{[]string{"read"}, map[string]float64{
			"alpha": 0.5442147286003255,
			"beta":  0,
			"gamma": 0.36928856583593517,
		}},
		{[]string{"file"}, map[string]float64{
			// Identical documents lengths give identical scores, and the longer
			// document is penalised by the b term for the same one occurrence.
			"alpha": 0.15461529672313143,
			"beta":  0.15461529672313143,
			"gamma": 0.1049175227764106,
		}},
		{[]string{"read", "disk"}, map[string]float64{
			"alpha": 0.5442147286003255,
			"beta":  0,
			"gamma": 1.1399401217737202,
		}},
		{[]string{"contents", "disk", "quickly"}, map[string]float64{
			"alpha": 0,
			"beta":  0,
			"gamma": 2.311954667813355,
		}},
	}
	names := []string{"alpha", "beta", "gamma"}
	for _, tc := range cases {
		for i, name := range names {
			got := ix.Score(i, tc.terms)
			if !close(got, tc.want[name]) {
				t.Errorf("Score(%s, %v) = %.17g, want %.17g", name, tc.terms, got, tc.want[name])
			}
		}
	}
}

// A document sharing no term with the query was not retrieved. Ranking it
// behind the matches would let a tool on a small surface count as "top 3"
// purely because nothing else was in the list.
func TestRankExcludesZeroScoringDocuments(t *testing.T) {
	ix := fixture()
	hits := ix.Rank([]string{"read"})
	if len(hits) != 2 {
		t.Fatalf("hits = %+v, want only the two documents containing the term", hits)
	}
	if hits[0].Name != "alpha" || hits[1].Name != "gamma" {
		t.Errorf("ranking = %+v, want alpha then gamma", hits)
	}
	if got := ix.RankOf([]string{"read"}, "beta"); got != 0 {
		t.Errorf("RankOf(beta) = %d, want 0 for a non-match", got)
	}
	if got := ix.RankOf([]string{"read"}, "gamma"); got != 2 {
		t.Errorf("RankOf(gamma) = %d, want 2", got)
	}
}

func TestRankBreaksTiesOnName(t *testing.T) {
	ix := fixture()
	hits := ix.Rank([]string{"file"})
	if len(hits) != 3 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Name != "alpha" || hits[1].Name != "beta" {
		t.Errorf("tied ranking = %+v, want the tie broken on name", hits)
	}
}

func TestEmptyIndexScoresNothing(t *testing.T) {
	ix := NewIndex(nil)
	if ix.Len() != 0 || ix.Score(0, []string{"read"}) != 0 || len(ix.Rank([]string{"read"})) != 0 {
		t.Error("an empty index produced a score")
	}
}

func TestTokenizeSplitsCasesAndSeparators(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"read_text_file", []string{"read", "text", "file"}},
		{"readTextFile", []string{"read", "text", "file"}},
		{"HTTPServerLog", []string{"http", "server", "log"}},
		{"browser-navigate_back", []string{"browser", "navigate", "back"}},
		{"Read a file.", []string{"read", "file"}}, // the single "a" is dropped
		{"", nil},
	}
	for _, tc := range cases {
		if got := Tokenize(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// One splitter serves the retrieval index and the hygiene naming check, so the
// two behaviours that differ between them are options rather than a fork. The
// camel and separator rule itself must be identical under every option.
func TestSplitIdentifierOptions(t *testing.T) {
	cases := []struct {
		in   string
		opts SplitOpts
		want []string
	}{
		{"read_text_file", SplitOpts{}, []string{"read", "text", "file"}},
		{"HTTPServerLog", SplitOpts{}, []string{"HTTP", "Server", "Log"}},
		// Retrieval keeps a digit run attached and drops short fragments.
		{"http2_connect", SplitOpts{}, []string{"http2", "connect"}},
		{"read_f", SplitOpts{}, []string{"read"}},
		// Dimension 5 needs both back: a bare number and a lone letter are the
		// fragments it reports.
		{"http2_connect", SplitOpts{DigitRuns: true, KeepShort: true}, []string{"http", "2", "connect"}},
		{"read_f", SplitOpts{DigitRuns: true, KeepShort: true}, []string{"read", "f"}},
		{"tool2", SplitOpts{DigitRuns: true, KeepShort: true}, []string{"tool", "2"}},
		{"HTTPServerLog", SplitOpts{DigitRuns: true, KeepShort: true}, []string{"HTTP", "Server", "Log"}},
		{"", SplitOpts{DigitRuns: true, KeepShort: true}, nil},
	}
	for _, tc := range cases {
		if got := SplitIdentifier(tc.in, tc.opts); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitIdentifier(%q, %+v) = %v, want %v", tc.in, tc.opts, got, tc.want)
		}
	}
}

// The derivation must not let a tool retrieve itself on its own name. Without
// the strip, a description that repeats the tool name would score a trivial
// self-match and the metric would measure nothing.
func TestDeriveStripsTheToolsOwnNameTokens(t *testing.T) {
	for _, name := range []string{"read_text_file", "readTextFile", "read-text-file"} {
		tool := Tool{
			Name:        name,
			Description: "Read the text of a file and return its contents to the caller.",
		}
		ix := NewIndex([]Doc{{Name: name, Text: tool.DescriptorText()}})
		tq := derive(tool, ix)
		if tq.Source != "description" {
			t.Fatalf("%s: source = %q", name, tq.Source)
		}
		if len(tq.Queries) == 0 {
			t.Fatalf("%s: no queries derived", name)
		}
		for _, q := range tq.Queries {
			for _, term := range q.Terms {
				if term == "read" || term == "text" || term == "file" {
					t.Errorf("%s: query %q kept the tool's own name token %q", name, q.Text, term)
				}
			}
		}
	}
}

func TestDeriveFormsAndDeterminism(t *testing.T) {
	tools := []Tool{
		{Name: "alpha_tool", Description: "Compile the project sources into a signed release bundle."},
		{Name: "beta_tool", Description: "Compile the project sources into a debug bundle for local runs."},
		{Name: "gamma_tool", Description: "Publish the project bundle to a remote artifact registry."},
	}
	ix := NewIndex([]Doc{
		{Name: tools[0].Name, Text: tools[0].DescriptorText()},
		{Name: tools[1].Name, Text: tools[1].DescriptorText()},
		{Name: tools[2].Name, Text: tools[2].DescriptorText()},
	})

	first := derive(tools[0], ix)
	if len(first.Queries) != QueriesPerTool {
		t.Fatalf("queries = %d, want %d: %+v", len(first.Queries), QueriesPerTool, first.Queries)
	}
	if first.Queries[0].Form != FormLeadingPhrase || first.Queries[0].Text != "compile project sources" {
		t.Errorf("leading phrase = %+v", first.Queries[0])
	}
	if first.Queries[1].Form != FormBigram {
		t.Errorf("second form = %q", first.Queries[1].Form)
	}
	// "signed release" is the rarest adjacent content pair on this surface:
	// "compile", "project", "sources", and "bundle" all appear on a sibling.
	if first.Queries[1].Text != "signed release" {
		t.Errorf("distinctive bigram = %q, want the highest-IDF adjacent pair", first.Queries[1].Text)
	}
	if first.Queries[2].Form != FormContentWords {
		t.Errorf("third form = %q", first.Queries[2].Form)
	}
	if first.Queries[2].Text != "compile project sources signed release bundle" {
		t.Errorf("content-word query = %q", first.Queries[2].Text)
	}

	for i := 0; i < 5; i++ {
		if !reflect.DeepEqual(derive(tools[0], ix), first) {
			t.Fatal("derivation is not deterministic across repeated calls")
		}
	}

	// Evaluate as a whole must also be stable, including the published figures.
	a := Evaluate("srv", tools)
	b := Evaluate("srv", tools)
	if !reflect.DeepEqual(a, b) {
		t.Error("Evaluate is not deterministic")
	}
}

func TestDeriveDeduplicatesCollidingForms(t *testing.T) {
	// A two-content-word description makes the leading phrase and the bigram
	// the same string; the set collapses rather than publishing a duplicate.
	tool := Tool{Name: "x_tool", Description: "Rotate credentials."}
	ix := NewIndex([]Doc{{Name: "x_tool", Text: tool.DescriptorText()}})
	tq := derive(tool, ix)
	seen := map[string]bool{}
	for _, q := range tq.Queries {
		if seen[q.Text] {
			t.Errorf("duplicate query %q in %+v", q.Text, tq.Queries)
		}
		seen[q.Text] = true
	}
}

// A tool with no prose to derive from is scored as not retrievable, not
// skipped. A gutted description is the failure this metric exists to catch, so
// dropping it from the denominator would reward it.
func TestUndescribedToolCountsAgainstTheScore(t *testing.T) {
	tools := []Tool{
		{Name: "alpha", Description: "Rotate the signing credentials for a deployment target."},
		{Name: "bare"},
	}
	res := Evaluate("srv", tools)
	if res.Score.Top3Fraction != 0.5 {
		t.Errorf("top3_fraction = %v, want 0.5", res.Score.Top3Fraction)
	}
	var bare ToolQueries
	for _, tq := range res.Set.Tools {
		if tq.Tool == "bare" {
			bare = tq
		}
	}
	if bare.Source != "none" || len(bare.Queries) != 0 || bare.Top3 {
		t.Errorf("undescribed tool = %+v", bare)
	}
}

func TestEvaluateFallsBackToTitle(t *testing.T) {
	tools := []Tool{
		{Name: "alpha", Title: "Rotate deployment signing credentials"},
		{Name: "beta", Description: "List every open pull request in a repository."},
	}
	res := Evaluate("srv", tools)
	if res.Set.Tools[0].Source != "title" {
		t.Errorf("source = %q, want the title fallback", res.Set.Tools[0].Source)
	}
	if !res.Set.Tools[0].Top1 {
		t.Error("a title-derived query did not retrieve its own tool")
	}
}

func TestEvaluatePublishesItsParameters(t *testing.T) {
	res := Evaluate("srv", []Tool{{Name: "alpha", Description: "Rotate the signing credentials."}})
	if res.Set.BM25K1 != K1 || res.Set.BM25B != B || res.Set.TopK != TopK {
		t.Errorf("query set did not stamp the pinned parameters: %+v", res.Set)
	}
	if res.Set.QueriesPerTool != QueriesPerTool || res.Score.QueriesPerTool != QueriesPerTool {
		t.Errorf("queries_per_tool = %d / %d", res.Set.QueriesPerTool, res.Score.QueriesPerTool)
	}
	if res.Score.Kind != KindMeasured {
		t.Errorf("kind = %q", res.Score.Kind)
	}
	if !strings.Contains(res.Set.Procedure, "derived mechanically") {
		t.Errorf("procedure note = %q", res.Set.Procedure)
	}
}

// MRR is taken over every derived query, including the ones that retrieved
// nothing. Averaging only the successful queries would report the mean rank of
// the queries that happened to work.
func TestMRRCountsUnmatchedQueries(t *testing.T) {
	tools := []Tool{
		{Name: "alpha", Description: "Rotate the signing credentials for a deployment target."},
		{Name: "bare"},
	}
	res := Evaluate("srv", tools)
	if res.Score.MRR <= 0 || res.Score.MRR > 1 {
		t.Fatalf("mrr = %v", res.Score.MRR)
	}
	// Three queries from alpha, all rank 1; none from bare.
	if res.Score.MRR != 1 {
		t.Errorf("mrr = %v, want 1: every derived query ranked its own tool first", res.Score.MRR)
	}
}

func close(a, b float64) bool { return math.Abs(a-b) < 1e-12 }
