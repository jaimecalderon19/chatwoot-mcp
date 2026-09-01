package chatwoot

import "testing"

func TestMergeLabelsPreservesExistingAndDedups(t *testing.T) {
	got := MergeLabels([]string{"existente", "humano"}, []string{"lead-caliente", "existente", "  "})
	want := []string{"existente", "humano", "lead-caliente"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestMergeLabelsCaseSensitive(t *testing.T) {
	got := MergeLabels([]string{"Lead"}, []string{"lead"})
	if len(got) != 2 || got[0] != "Lead" || got[1] != "lead" {
		t.Fatalf("got %v", got)
	}
}

func TestSubtractLabels(t *testing.T) {
	got := SubtractLabels([]string{"a", "b", "c", "b"}, []string{"b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("got %v", got)
	}
}
