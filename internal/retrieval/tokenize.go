package retrieval

import (
	"strings"
	"unicode"
)

// SplitOpts selects the two behaviours that differ between the callers of
// SplitIdentifier. They are options rather than separate functions because the
// splitting rule itself must not fork: the retrieval index and the hygiene
// naming check have to agree on where one fragment ends and the next begins,
// or the two published metrics disagree about what a word is.
type SplitOpts struct {
	// DigitRuns makes a run of digits its own fragment, so "tool2" splits into
	// "tool" and "2". Retrieval leaves it off, because "http2" is one term to a
	// reader typing a query. Dimension 5 of the hygiene grade turns it on,
	// because a bare numeric fragment is precisely what it looks for.
	DigitRuns bool
	// KeepShort keeps fragments shorter than two runes. Retrieval drops them:
	// they carry no retrieval signal, and a stray "a" or "1" would otherwise
	// pick up an inverse-document-frequency weight. Dimension 5 keeps them,
	// because a single-letter fragment is the defect it reports.
	KeepShort bool
}

// SplitIdentifier breaks text into fragments at every non-alphanumeric rune and
// at camelCase boundaries, so `read_text_file` and `readTextFile` both split
// into the same three fragments and a tool name cannot hide behind its own
// casing convention. Case is preserved; callers that index the result lowercase
// it themselves.
func SplitIdentifier(s string, opts SplitOpts) []string {
	var out []string
	for _, chunk := range strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		for _, piece := range splitChunk(chunk, opts.DigitRuns) {
			if !opts.KeepShort && len([]rune(piece)) < 2 {
				continue
			}
			out = append(out, piece)
		}
	}
	return out
}

// Tokenize lowercases text and splits it into index terms with the shared
// splitter, dropping single-character tokens.
//
// There is no stemming. A stemmer is a second dictionary to pin, version, and
// defend, and the queries this package derives come from the same vocabulary as
// the documents they are scored against, so stemming would change few outcomes
// while adding a dependency the methodology would have to version.
func Tokenize(s string) []string {
	var out []string
	for _, piece := range SplitIdentifier(s, SplitOpts{}) {
		out = append(out, strings.ToLower(piece))
	}
	return out
}

// splitChunk breaks one alphanumeric chunk at camelCase boundaries. It splits
// before an uppercase rune that follows a lowercase rune or a digit, and before
// the last uppercase rune of a run that is followed by a lowercase rune, so
// "HTTPServer" yields "HTTP" and "Server" rather than one opaque token. With
// digitRuns set it also splits at every letter-to-digit transition.
func splitChunk(chunk string, digitRuns bool) []string {
	runes := []rune(chunk)
	if len(runes) < 2 {
		return []string{chunk}
	}
	var out []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev, cur := runes[i-1], runes[i]
		boundary := false
		if unicode.IsUpper(cur) {
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			boundary = unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower)
		}
		if !boundary && digitRuns {
			boundary = unicode.IsDigit(cur) != unicode.IsDigit(prev)
		}
		if boundary {
			out = append(out, string(runes[start:i]))
			start = i
		}
	}
	return append(out, string(runes[start:]))
}

// stopwords is a closed, published list. It is deliberately short and generic:
// a longer list tuned against this corpus would be a hidden parameter fitted to
// the servers being measured.
var stopwords = map[string]bool{
	"about": true, "above": true, "after": true, "against": true, "all": true,
	"also": true, "an": true, "and": true, "any": true, "are": true, "as": true,
	"at": true, "be": true, "been": true, "before": true, "being": true,
	"below": true, "between": true, "both": true, "but": true, "by": true,
	"can": true, "do": true, "does": true, "each": true, "either": true,
	"for": true, "from": true, "further": true, "had": true, "has": true,
	"have": true, "if": true, "in": true, "into": true, "is": true, "it": true,
	"its": true, "may": true, "more": true, "most": true, "must": true,
	"no": true, "nor": true, "not": true, "of": true, "off": true, "on": true,
	"once": true, "only": true, "or": true, "other": true, "our": true,
	"out": true, "over": true, "own": true, "per": true, "same": true,
	"should": true, "so": true, "some": true, "such": true, "than": true,
	"that": true, "the": true, "their": true, "them": true, "then": true,
	"there": true, "these": true, "they": true, "this": true, "those": true,
	"through": true, "to": true, "too": true, "under": true, "until": true,
	"up": true, "use": true, "used": true, "using": true, "very": true,
	"was": true, "we": true, "were": true, "what": true, "when": true,
	"where": true, "which": true, "while": true, "who": true, "will": true,
	"with": true, "would": true, "you": true, "your": true,
}

// IsStopword reports whether a token is on the published stopword list.
func IsStopword(token string) bool { return stopwords[token] }

// ContentWords tokenizes text and drops stopwords, preserving order and
// repetition. It is the shared basis for query derivation here and for the
// descriptor-similarity check in the hygiene grade, so the two metrics cannot
// disagree about what a content word is.
func ContentWords(s string) []string {
	toks := Tokenize(s)
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if stopwords[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}
