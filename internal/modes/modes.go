// Package modes turns measured token counts into the three client-mode
// figures of methodology 3. Every figure carries a measured or modeled kind,
// because a single cost number is a wrong number.
package modes

// Kind labels a figure as coming from a harness run or from a formula.
const (
	KindMeasured = "measured"
	KindModeled  = "modeled"
)

// PublishedStubTokens is the upfront tool-search facility cost. Methodology
// 3.2 takes the published figure where the stub format is not published, so
// this is modeled, not measured.
const PublishedStubTokens = 500

// PublishedCodeModeTokens is the published full-surface code-mode figure from
// methodology 3.3.
const PublishedCodeModeTokens = 1000

// KMin and KMax bound the tools retrieved per session in methodology 3.2.
const (
	KMin = 3
	KMax = 5
)

// Naive is the full-load figure: the canonical serialization total.
type Naive struct {
	Tokens int    `json:"tokens"`
	Kind   string `json:"kind"`
}

// ToolSearch is the progressive-disclosure figure. Its inputs are measured and
// its total is modeled, because k is an assumption.
type ToolSearch struct {
	StubTokens int    `json:"stub_tokens"`
	PerToolAvg int    `json:"per_tool_avg"`
	KRange     [2]int `json:"k_range"`
	Kind       string `json:"kind"`
}

// CodeMode is the programmatic-interface figure, modeled until a Tier 2 run
// validates it against real call traffic.
type CodeMode struct {
	TokensEstimate int    `json:"tokens_estimate"`
	Kind           string `json:"kind"`
}

// Set is the three modes for one server.
type Set struct {
	Naive      Naive      `json:"naive"`
	ToolSearch ToolSearch `json:"tool_search"`
	CodeMode   CodeMode   `json:"code_mode"`
}

// Compute derives the three modes from the canonical-serialization total and
// the per-tool counts. It is called only when the o200k_base count actually
// succeeded: a row with no o200k figure publishes no mode block at all, rather
// than a zero labelled "measured". There is deliberately no zero-valued
// constructor here, because every value one could return would be a figure the
// harness did not measure.
func Compute(canonicalTotal int, perTool []int) Set {
	return Set{
		Naive: Naive{Tokens: canonicalTotal, Kind: KindMeasured},
		ToolSearch: ToolSearch{
			StubTokens: PublishedStubTokens,
			PerToolAvg: mean(perTool),
			KRange:     [2]int{KMin, KMax},
			Kind:       KindModeled,
		},
		CodeMode: CodeMode{
			TokensEstimate: codeModeEstimate(canonicalTotal),
			Kind:           KindModeled,
		},
	}
}

// ProgressiveTotal applies C_progressive = C_stub + sum(C_tool_i) at a given k
// using the per-tool average. It is exported for the calculator rather than
// stored per row, since the row publishes the inputs and the k range.
func (t ToolSearch) ProgressiveTotal(k int) int {
	if k < 0 {
		k = 0
	}
	return t.StubTokens + k*t.PerToolAvg
}

// codeModeEstimate takes the published full-surface figure, clamped so it can
// never exceed the naive count it is meant to replace. The clamp is the one
// harness addition to the published behaviour and only binds on surfaces
// already smaller than the published figure.
func codeModeEstimate(canonicalTotal int) int {
	if canonicalTotal > 0 && canonicalTotal < PublishedCodeModeTokens {
		return canonicalTotal
	}
	return PublishedCodeModeTokens
}

func mean(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return sum / len(xs)
}
