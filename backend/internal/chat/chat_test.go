package chat

import (
	"strings"
	"testing"
)

func TestFilterMasksBannedWords(t *testing.T) {
	f := NewFilter([]string{"badword"})
	got := f.Clean("you are a badword player")
	want := "you are a ******* player"
	if got != want {
		t.Errorf("Clean() = %q, want %q", got, want)
	}
}

func TestFilterCatchesSimpleObfuscation(t *testing.T) {
	f := NewFilter([]string{"badword"})
	// Punctuation between letters must not defeat the filter; the whole
	// token is masked, punctuation included.
	got := f.Clean("b.a.d.w.o.r.d")
	if got != strings.Repeat("*", len("b.a.d.w.o.r.d")) {
		t.Errorf("Clean() = %q, want the whole token masked", got)
	}
}

func TestFilterIsCaseInsensitive(t *testing.T) {
	f := NewFilter([]string{"badword"})
	if got := f.Clean("BadWord"); got != "*******" {
		t.Errorf("Clean() = %q, want fully masked", got)
	}
}

func TestFilterLeavesCleanTextUntouched(t *testing.T) {
	f := NewFilter([]string{"badword"})
	in := "Good game, thanks! 好棋"
	if got := f.Clean(in); got != in {
		t.Errorf("Clean() modified clean text: %q", got)
	}
}

func TestFilterDoesNotCensorGoVocabulary(t *testing.T) {
	f := NewFilter([]string{"ass"})
	// "pass" contains "ass". Censoring it would be worse than missing the
	// insult, since passing is a move in every game.
	for _, in := range []string{"I pass", "pass please", "atari", "Class"} {
		if got := f.Clean(in); got != in {
			t.Errorf("Clean(%q) = %q, want it untouched", in, got)
		}
	}
	// The standalone word is still caught.
	if got := f.Clean("you ass"); got != "you ***" {
		t.Errorf("Clean(\"you ass\") = %q, want \"you ***\"", got)
	}
}

func TestFilterCatchesLeetSubstitutions(t *testing.T) {
	f := NewFilter([]string{"shit"})
	for _, in := range []string{"sh1t", "5h!t", "SH1T"} {
		if got := f.Clean(in); got != strings.Repeat("*", len(in)) {
			t.Errorf("Clean(%q) = %q, want fully masked", in, got)
		}
	}
}

func TestFilterMasksOnlyTheOffendingToken(t *testing.T) {
	f := NewFilter([]string{"badword"})
	got := f.Clean("nice badword game")
	if got != "nice ******* game" {
		t.Errorf("Clean() = %q, want only the middle token masked", got)
	}
}
