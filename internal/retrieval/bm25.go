package retrieval

import (
	"math"
	"sort"
)

// BM25 parameters, pinned and published per methodology 3.2. They are the
// standard Robertson/Sparck-Jones defaults; they are stated here rather than
// tuned, because a parameter fitted against the measured corpus would make the
// score a property of this corpus instead of a property of the server.
const (
	K1 = 1.2
	B  = 0.75
)

// Doc is one indexed document: a tool, addressed by name.
type Doc struct {
	Name string
	Text string
}

// Index is a BM25 index over one server's own tool surface. Methodology 5
// indexes each server separately: a real client searches a merged corpus across
// every attached server, and this index deliberately measures the easier
// within-server problem.
type Index struct {
	names []string
	tf    []map[string]int
	dl    []int
	df    map[string]int
	avgdl float64
}

// NewIndex builds the index in document order.
func NewIndex(docs []Doc) *Index {
	ix := &Index{df: map[string]int{}}
	total := 0
	for _, d := range docs {
		toks := Tokenize(d.Text)
		tf := map[string]int{}
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			ix.df[t]++
		}
		ix.names = append(ix.names, d.Name)
		ix.tf = append(ix.tf, tf)
		ix.dl = append(ix.dl, len(toks))
		total += len(toks)
	}
	if len(docs) > 0 {
		ix.avgdl = float64(total) / float64(len(docs))
	}
	return ix
}

// Len is the number of indexed documents.
func (ix *Index) Len() int { return len(ix.names) }

// IDF is the BM25 inverse document frequency, in the non-negative form
// ln(1 + (N - df + 0.5) / (df + 0.5)).
func (ix *Index) IDF(term string) float64 {
	n := float64(len(ix.tf))
	df := float64(ix.df[term])
	return math.Log(1 + (n-df+0.5)/(df+0.5))
}

// Score is the BM25 score of one document against a term list.
func (ix *Index) Score(doc int, terms []string) float64 {
	if ix.avgdl == 0 || doc < 0 || doc >= len(ix.tf) {
		return 0
	}
	norm := K1 * (1 - B + B*float64(ix.dl[doc])/ix.avgdl)
	var sum float64
	for _, t := range terms {
		f := float64(ix.tf[doc][t])
		if f == 0 {
			continue
		}
		sum += ix.IDF(t) * f * (K1 + 1) / (f + norm)
	}
	return sum
}

// Hit is one ranked document.
type Hit struct {
	Name  string
	Score float64
}

// Rank returns the documents that score above zero, best first. Zero-scoring
// documents are omitted rather than ranked behind the others: a document that
// shares no term with the query was not retrieved, and padding the tail with
// non-matches would let a tool count as "top 3" on a two-tool server purely
// because there was nothing else in the list. Ties break on document name so
// the ranking is deterministic across runs.
func (ix *Index) Rank(terms []string) []Hit {
	var hits []Hit
	for i := range ix.names {
		s := ix.Score(i, terms)
		if s <= 0 {
			continue
		}
		hits = append(hits, Hit{Name: ix.names[i], Score: s})
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		return hits[a].Name < hits[b].Name
	})
	return hits
}

// RankOf returns the 1-based rank of the named document for a query, or 0 when
// the document did not match the query at all.
func (ix *Index) RankOf(terms []string, name string) int {
	for i, h := range ix.Rank(terms) {
		if h.Name == name {
			return i + 1
		}
	}
	return 0
}
