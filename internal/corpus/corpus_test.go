package corpus

import (
	"path/filepath"
	"reflect"
	"testing"
)

// Go randomizes map iteration, so a flag entry with more than one key emitted
// corpus_notes in a different order on every run of an unchanged corpus. The
// published row has to be a function of the corpus alone.
func TestNotesAreDeterministic(t *testing.T) {
	s := Server{Flags: []map[string]any{
		{
			"auth-verification": "classic-PAT-scope-filtering",
			"resolvable":        "likely, via OAuth flow instead of a classic PAT",
			"note":              "third key, so a two-key entry cannot pass by luck",
		},
		{"note": "second entry keeps its position"},
	}}

	first := s.Notes()
	want := []string{
		"auth-verification: classic-PAT-scope-filtering",
		"note: third key, so a two-key entry cannot pass by luck",
		"resolvable: likely, via OAuth flow instead of a classic PAT",
		"note: second entry keeps its position",
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("notes = %v, want %v", first, want)
	}
	// Repeat enough times that a randomized order would have shown up.
	for i := 0; i < 200; i++ {
		if !reflect.DeepEqual(s.Notes(), first) {
			t.Fatalf("notes changed between calls: %v then %v", first, s.Notes())
		}
	}
}

// The multi-key flags in the shipped corpus are the ones that actually moved.
func TestShippedCorpusNotesAreDeterministic(t *testing.T) {
	file, err := Load(filepath.Join("..", "..", "servers.yaml"))
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	multiKey := 0
	for _, s := range file.Servers {
		for _, f := range s.Flags {
			if len(f) > 1 {
				multiKey++
			}
		}
		first := s.Notes()
		for i := 0; i < 50; i++ {
			if !reflect.DeepEqual(s.Notes(), first) {
				t.Fatalf("%s notes changed between calls: %v then %v", s.ID, first, s.Notes())
			}
		}
	}
	if multiKey == 0 {
		t.Error("no multi-key flag entry left in the corpus, so this test proves nothing")
	}
}
