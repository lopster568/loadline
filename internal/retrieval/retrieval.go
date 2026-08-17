// Package retrieval scores whether a tool can be found. Under a tool-search
// client a tool that never surfaces for a plausible task phrase is effectively
// broken, however cheap its schema is, so methodology 5 scores retrieval
// alongside cost.
//
// Queries are derived mechanically from each tool's own description. The
// alternative, a human author writing task phrases per tool, was rejected: a
// public ranking cannot rest on queries whose authorship is the ranking
// author's discretion. The cost of the mechanical rule is stated plainly in
// methodology 5 and repeated here: same-source derivation measures within-server
// disambiguation, that is, whether sibling tools shadow each other, and not
// whether real user phrasing finds the tool. The full derived query set is
// published per server so the derivation can be inspected rather than trusted.
package retrieval

import "strings"

// QueriesPerTool is N in methodology 5: three query forms per tool.
const QueriesPerTool = 3

// TopK is the published cutoff. A tool counts as retrievable when at least one
// of its derived queries ranks it in the top TopK of its own server's surface.
const TopK = 3

// leadingPhraseLen is the number of leading content words taken as the
// approximate imperative verb phrase.
const leadingPhraseLen = 3

// contentWordsCap bounds the full content-word query. Beyond a dozen terms the
// query stops resembling anything a user would type and starts being a copy of
// the document.
const contentWordsCap = 12

// Query form identifiers, published in the per-server query artifact.
const (
	FormLeadingPhrase = "leading_phrase"
	FormBigram        = "distinctive_bigram"
	FormContentWords  = "content_words"
)

// Tool is the input surface: the three descriptor fields methodology 5 indexes.
type Tool struct {
	Name        string
	Title       string
	Description string
	// Descriptor is the canonicalizer's join of the three fields above, which
	// methodology 5 names as the indexed document text. It is carried through
	// from canon.Tool rather than rebuilt here, so the document the score is
	// computed over is the same string the canonicalizer published. It may be
	// left empty by a caller holding no canonical surface, in which case
	// DescriptorText falls back to the shared join.
	Descriptor string
}

// Descriptor joins name, title, and description into the indexed document text
// of methodology 5, skipping the fields a tool did not declare. It is exported
// because the canonicalizer builds the authoritative descriptor with it and the
// hygiene grade tokenizes the result: one join, three callers, no drift.
func Descriptor(name, title, description string) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{name, title, description} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

// DescriptorText returns the canonicalizer's descriptor when the caller
// supplied one, and the shared join otherwise.
func (t Tool) DescriptorText() string {
	if t.Descriptor != "" {
		return t.Descriptor
	}
	return Descriptor(t.Name, t.Title, t.Description)
}

// Query is one derived query and the rank its source tool reached.
type Query struct {
	Form  string   `json:"form"`
	Text  string   `json:"text"`
	Terms []string `json:"terms"`
	// Rank is 1-based, or 0 when the source tool did not match its own query
	// at all, which happens only when every derived term is shared with no
	// document including the source.
	Rank int `json:"rank"`
}

// ToolQueries is the derived query set and outcome for one tool.
type ToolQueries struct {
	Tool string `json:"tool"`
	// Source names the descriptor field the queries were derived from:
	// "description", "title" when the description is empty, or "none" when the
	// tool offers no text to derive from. A "none" tool is scored as not
	// retrievable rather than skipped, because an undescribed tool is exactly
	// the failure this metric exists to catch.
	Source  string  `json:"source"`
	Queries []Query `json:"queries"`
	Top1    bool    `json:"top1"`
	Top3    bool    `json:"top3"`
}

// QuerySet is the published derivation record for one server, written to
// data/runs/<date>/<server>-queries.json so a critic can inspect the queries
// the score was computed from.
type QuerySet struct {
	Server         string        `json:"server"`
	Procedure      string        `json:"procedure"`
	QueriesPerTool int           `json:"queries_per_tool"`
	TopK           int           `json:"top_k"`
	BM25K1         float64       `json:"bm25_k1"`
	BM25B          float64       `json:"bm25_b"`
	Tools          []ToolQueries `json:"tools"`
}

// Score is the published per-server retrievability block.
type Score struct {
	// Top3Fraction is the published figure: the fraction of tools that at
	// least one of their own derived queries ranks in the top three of their
	// own server's surface.
	Top3Fraction float64 `json:"top3_fraction"`
	// MRR is the secondary figure: the mean reciprocal rank over every derived
	// query. On a surface of three or fewer tools the top-three fraction is
	// close to vacuous, and MRR is the figure that still separates a tool that
	// ranks first from one that ranks third.
	MRR            float64 `json:"mrr"`
	QueriesPerTool int     `json:"queries_per_tool"`
	Kind           string  `json:"kind"`
}

// Result is the score plus the derivation record backing it.
type Result struct {
	Score Score
	Set   QuerySet
}

const procedureNote = "queries derived mechanically from each tool's own description: stopwords and the tool's own name fragments stripped, three forms per tool (leading content-word phrase, highest-IDF adjacent content-word bigram, deduplicated content-word set)"

// Evaluate derives queries for every tool, scores them with BM25 over the same
// server's surface, and returns both the published figures and the derivation
// record. It contacts nothing: the input is the already-enumerated surface.
func Evaluate(server string, tools []Tool) Result {
	set := QuerySet{
		Server:         server,
		Procedure:      procedureNote,
		QueriesPerTool: QueriesPerTool,
		TopK:           TopK,
		BM25K1:         K1,
		BM25B:          B,
	}
	docs := make([]Doc, 0, len(tools))
	for _, t := range tools {
		docs = append(docs, Doc{Name: t.Name, Text: t.DescriptorText()})
	}
	ix := NewIndex(docs)

	top3 := 0
	var rrSum float64
	rrCount := 0
	for _, t := range tools {
		tq := derive(t, ix)
		for i := range tq.Queries {
			rank := ix.RankOf(tq.Queries[i].Terms, t.Name)
			tq.Queries[i].Rank = rank
			if rank > 0 {
				rrSum += 1 / float64(rank)
			}
			rrCount++
			if rank == 1 {
				tq.Top1 = true
			}
			if rank >= 1 && rank <= TopK {
				tq.Top3 = true
			}
		}
		if tq.Top3 {
			top3++
		}
		set.Tools = append(set.Tools, tq)
	}

	sc := Score{QueriesPerTool: QueriesPerTool, Kind: KindMeasured}
	if len(tools) > 0 {
		sc.Top3Fraction = Round4(float64(top3) / float64(len(tools)))
	}
	if rrCount > 0 {
		sc.MRR = Round4(rrSum / float64(rrCount))
	}
	return Result{Score: sc, Set: set}
}

// KindMeasured labels the block as harness output rather than formula output.
// Both figures come from the enumerated surface with no assumed parameter, so
// unlike the client modes of methodology 3 there is nothing modeled here.
const KindMeasured = "measured"

// derive builds the query set for one tool. It is deterministic: the same tool
// and the same index always produce the same queries in the same order.
func derive(t Tool, ix *Index) ToolQueries {
	tq := ToolQueries{Tool: t.Name, Source: "none"}

	src := strings.TrimSpace(t.Description)
	if src != "" {
		tq.Source = "description"
	} else if src = strings.TrimSpace(t.Title); src != "" {
		tq.Source = "title"
	} else {
		return tq
	}

	// Stripping the tool's own name fragments is what stops the derivation from
	// being a trivial self-match: without it, a description that repeats the
	// tool name retrieves itself on the name field alone and the metric would
	// measure nothing but whether the author wrote the name twice.
	own := map[string]bool{}
	for _, tok := range Tokenize(t.Name) {
		own[tok] = true
	}
	var content []string
	for _, w := range ContentWords(src) {
		if own[w] {
			continue
		}
		content = append(content, w)
	}
	if len(content) == 0 {
		return tq
	}

	add := func(form string, terms []string) {
		if len(terms) == 0 {
			return
		}
		text := strings.Join(terms, " ")
		for _, q := range tq.Queries {
			if q.Text == text {
				return
			}
		}
		tq.Queries = append(tq.Queries, Query{Form: form, Text: text, Terms: terms})
	}

	// Form 1: the leading content-word phrase. MCP descriptions conventionally
	// open with an imperative verb ("Read the complete contents of a file"), so
	// the first few content words approximate the leading verb phrase without
	// requiring a part-of-speech tagger, which would be another dictionary to
	// pin and version.
	n := leadingPhraseLen
	if len(content) < n {
		n = len(content)
	}
	add(FormLeadingPhrase, append([]string(nil), content[:n]...))

	// Form 2: the most distinctive adjacent content-word bigram, where
	// distinctiveness is the summed IDF of the pair over this server's own
	// surface. Adjacency is measured after stopword removal, so "contents of a
	// file" yields the pair "contents file". Ties go to the earliest pair.
	if len(content) >= 2 {
		best, bestScore := 0, -1.0
		for i := 0; i+1 < len(content); i++ {
			s := ix.IDF(content[i]) + ix.IDF(content[i+1])
			if s > bestScore {
				best, bestScore = i, s
			}
		}
		add(FormBigram, []string{content[best], content[best+1]})
	}

	// Form 3: the deduplicated content-word set, capped so a long description
	// does not turn its query into a copy of the document.
	seen := map[string]bool{}
	var full []string
	for _, w := range content {
		if seen[w] {
			continue
		}
		seen[w] = true
		full = append(full, w)
		if len(full) == contentWordsCap {
			break
		}
	}
	add(FormContentWords, full)

	return tq
}

// Round4 is the shared four-decimal rounding used by every published fraction
// in methodology 5 and 6. It lives here so the two packages cannot round the
// same figure differently.
func Round4(f float64) float64 {
	return float64(int64(f*10000+0.5)) / 10000
}
