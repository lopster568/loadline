package modes

import "testing"

func TestComputeLabelsEveryMode(t *testing.T) {
	set := Compute(12000, []int{300, 400, 500})

	if set.Naive.Kind != KindMeasured {
		t.Errorf("naive kind = %q, want measured", set.Naive.Kind)
	}
	if set.Naive.Tokens != 12000 {
		t.Errorf("naive tokens = %d", set.Naive.Tokens)
	}
	if set.ToolSearch.Kind != KindModeled {
		t.Errorf("tool_search kind = %q, want modeled because k is an assumption", set.ToolSearch.Kind)
	}
	if set.ToolSearch.PerToolAvg != 400 {
		t.Errorf("per_tool_avg = %d, want 400", set.ToolSearch.PerToolAvg)
	}
	if set.ToolSearch.KRange != [2]int{3, 5} {
		t.Errorf("k_range = %v", set.ToolSearch.KRange)
	}
	if set.ToolSearch.StubTokens != PublishedStubTokens {
		t.Errorf("stub_tokens = %d", set.ToolSearch.StubTokens)
	}
	if set.CodeMode.Kind != KindModeled {
		t.Errorf("code_mode kind = %q, want modeled until Tier 2 validates it", set.CodeMode.Kind)
	}
	if set.CodeMode.TokensEstimate != PublishedCodeModeTokens {
		t.Errorf("code_mode estimate = %d", set.CodeMode.TokensEstimate)
	}
}

func TestProgressiveTotalUsesTheFormula(t *testing.T) {
	set := Compute(12000, []int{300, 400, 500})
	if got := set.ToolSearch.ProgressiveTotal(3); got != 500+3*400 {
		t.Errorf("k=3 total = %d", got)
	}
	if got := set.ToolSearch.ProgressiveTotal(5); got != 500+5*400 {
		t.Errorf("k=5 total = %d", got)
	}
	if got := set.ToolSearch.ProgressiveTotal(0); got != 500 {
		t.Errorf("k=0 total = %d", got)
	}
}

func TestCodeModeClampedToNaive(t *testing.T) {
	if got := Compute(250, []int{250}).CodeMode.TokensEstimate; got != 250 {
		t.Errorf("code_mode estimate = %d, want the naive count on a tiny surface", got)
	}
	if got := Compute(0, nil).CodeMode.TokensEstimate; got != PublishedCodeModeTokens {
		t.Errorf("code_mode estimate on an empty surface = %d", got)
	}
}

func TestUnmeasuredCarriesNoTokenFigures(t *testing.T) {
	set := Unmeasured()
	if set.Naive.Tokens != 0 || set.ToolSearch.StubTokens != 0 ||
		set.ToolSearch.PerToolAvg != 0 || set.CodeMode.TokensEstimate != 0 {
		t.Errorf("failure row published token figures: %+v", set)
	}
	if set.Naive.Kind != KindMeasured || set.ToolSearch.Kind != KindModeled || set.CodeMode.Kind != KindModeled {
		t.Errorf("failure row lost its kind labels: %+v", set)
	}
	if set.ToolSearch.KRange != [2]int{KMin, KMax} {
		t.Errorf("k_range = %v", set.ToolSearch.KRange)
	}
}

func TestEmptySurfaceHasNoPerToolAverage(t *testing.T) {
	if got := Compute(0, nil).ToolSearch.PerToolAvg; got != 0 {
		t.Errorf("per_tool_avg = %d, want 0", got)
	}
}
