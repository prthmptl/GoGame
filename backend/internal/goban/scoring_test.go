package goban

import "testing"

// splitPosition builds a clean 9x9 where black owns the left third, white the
// right third, and the middle column is dame:
//
//	cols 0-2  empty  -> black territory (27)
//	col  3    black wall (9 stones)
//	col  4    empty  -> neutral, touches both (9)
//	col  5    white wall (9 stones)
//	cols 6-8  empty  -> white territory (27)
func splitPosition(t *testing.T, r Ruleset) *State {
	t.Helper()
	s := NewGame(NewConfig(9, r, 0))
	b := s.Board
	for row := 0; row < 9; row++ {
		b = b.Set(Point{row, 3}, Black)
		b = b.Set(Point{row, 5}, White)
	}
	s.Board = b
	return s
}

func TestEmptyBoardIsAllNeutral(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	sc := ScorePosition(s, nil)
	if sc.Neutral != 81 {
		t.Errorf("neutral = %d, want 81 on an empty board", sc.Neutral)
	}
	if sc.BlackTerritory != 0 || sc.WhiteTerritory != 0 {
		t.Error("empty board should award no territory")
	}
	// Nobody has anything, so white takes it on komi.
	if sc.Winner() != "white" {
		t.Errorf("winner = %s, want white by komi", sc.Winner())
	}
}

func TestTerritoryIsBoundedByBothColours(t *testing.T) {
	sc := ScorePosition(splitPosition(t, Chinese), nil)
	if sc.BlackStones != 9 || sc.WhiteStones != 9 {
		t.Errorf("stones = %d/%d, want 9/9", sc.BlackStones, sc.WhiteStones)
	}
	if sc.BlackTerritory != 27 {
		t.Errorf("black territory = %d, want 27", sc.BlackTerritory)
	}
	if sc.WhiteTerritory != 27 {
		t.Errorf("white territory = %d, want 27", sc.WhiteTerritory)
	}
	// The column between the two walls touches both, so it is dame.
	if sc.Neutral != 9 {
		t.Errorf("neutral = %d, want 9 (the dame column)", sc.Neutral)
	}
	if total := sc.BlackStones + sc.WhiteStones + sc.BlackTerritory +
		sc.WhiteTerritory + sc.Neutral; total != 81 {
		t.Errorf("points accounted for = %d, want 81", total)
	}
}

func TestAreaVersusTerritoryScoring(t *testing.T) {
	area := ScorePosition(splitPosition(t, Chinese), nil)  // komi 7.5
	terr := ScorePosition(splitPosition(t, Japanese), nil) // komi 6.5

	// Area counts stones plus territory.
	if got := area.BlackTotal(); got != 36 {
		t.Errorf("area black total = %v, want 36 (9 stones + 27 territory)", got)
	}
	if got := area.WhiteTotal(); got != 43.5 {
		t.Errorf("area white total = %v, want 43.5 (36 + 7.5 komi)", got)
	}
	// Territory counts territory plus prisoners, ignoring living stones.
	if got := terr.BlackTotal(); got != 27 {
		t.Errorf("territory black total = %v, want 27", got)
	}
	if got := terr.WhiteTotal(); got != 33.5 {
		t.Errorf("territory white total = %v, want 33.5 (27 + 6.5 komi)", got)
	}
}

func TestDeadStonesBecomeTerritoryAndPrisoners(t *testing.T) {
	s := splitPosition(t, Japanese)
	// A white stone stranded deep in black's area.
	s.Board = s.Board.Set(Point{0, 0}, White)

	alive := ScorePosition(s, nil)
	// While it is treated as alive, black's region touches white too, so the
	// whole region becomes dame rather than territory.
	if alive.BlackTerritory != 0 {
		t.Errorf("black territory = %d, want 0 while the invader counts as alive",
			alive.BlackTerritory)
	}

	dead := ScorePosition(s, []Point{{0, 0}})
	if dead.BlackTerritory != 27 {
		t.Errorf("black territory = %d, want 27 once the stone is marked dead",
			dead.BlackTerritory)
	}
	if dead.BlackPrisoners != 1 {
		t.Errorf("black prisoners = %d, want 1 (the dead white stone)", dead.BlackPrisoners)
	}
	if dead.WhiteStones != 9 {
		t.Errorf("white stones = %d, want 9 after removing the dead one", dead.WhiteStones)
	}
}

func TestResultString(t *testing.T) {
	cases := []struct {
		name string
		sc   Score
		want string
	}{
		{
			"black ahead",
			Score{Method: TerritoryScoring, BlackTerritory: 10, Komi: 2.5},
			"B+7.5",
		},
		{
			"white ahead",
			Score{Method: TerritoryScoring, WhiteTerritory: 1, Komi: 2.5},
			"W+3.5",
		},
		{
			"drawn",
			Score{Method: TerritoryScoring, BlackTerritory: 5, WhiteTerritory: 5, Komi: 0},
			"Draw",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sc.Result(); got != c.want {
				t.Errorf("Result() = %s, want %s (margin %v)", got, c.want, c.sc.Margin())
			}
		})
	}
}

func TestWinnerMatchesMargin(t *testing.T) {
	sc := ScorePosition(splitPosition(t, Chinese), nil)
	if sc.Margin() >= 0 {
		t.Fatalf("expected white ahead by komi, margin = %v", sc.Margin())
	}
	if sc.Winner() != "white" {
		t.Errorf("winner = %s, want white", sc.Winner())
	}
}
