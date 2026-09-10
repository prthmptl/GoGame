package goban

import (
	"encoding/json"
	"os"
	"testing"
)

// The Flutter tests execute this same file, including the expected boards.
func TestSharedRulesFixtures(t *testing.T) {
	raw, err := os.ReadFile("testdata/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name     string
		Size     int
		Rules    Ruleset
		Handicap int
		Steps    []struct {
			Kind     MoveKind
			Row, Col int
			Accept   bool
		}
		Black, White   [][2]int
		ToMove, Status string
		Moves          int
		Captures       [2]int
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			state := NewGame(NewConfig(f.Size, f.Rules, f.Handicap))
			for i, step := range f.Steps {
				intent := Intent{Kind: step.Kind}
				if step.Kind == Place {
					intent = PlaceAt(Point{step.Row, step.Col})
				}
				next, _, err := state.Apply(intent)
				if (err == nil) != step.Accept {
					t.Fatalf("step %d: acceptance=%v want=%v error=%v", i, err == nil, step.Accept, err)
				}
				if err == nil {
					state = next
				}
			}
			expected := NewBoard(f.Size)
			for _, p := range f.Black {
				expected = expected.Set(Point{p[0], p[1]}, Black)
			}
			for _, p := range f.White {
				expected = expected.Set(Point{p[0], p[1]}, White)
			}
			for r := 0; r < f.Size; r++ {
				for c := 0; c < f.Size; c++ {
					p := Point{r, c}
					if state.Board.At(p) != expected.At(p) {
						t.Fatalf("board differs at %v", p)
					}
				}
			}
			toMove := "black"
			if state.ToMove == White {
				toMove = "white"
			}
			if toMove != f.ToMove || string(state.Status) != f.Status || state.MoveNumber != f.Moves || state.CapturesByBlack != f.Captures[0] || state.CapturesByWhite != f.Captures[1] {
				t.Fatalf("unexpected final state: %+v", state)
			}
		})
	}
}
