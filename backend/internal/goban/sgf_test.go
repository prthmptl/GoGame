package goban

import "testing"

func TestRoundTripSGF(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	for _, p := range []Point{{4, 4}, {2, 2}, {6, 6}} {
		next, _, err := s.Apply(PlaceAt(p))
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		s = next
	}
	sgf := ExportSGF(s, SGFMeta{BlackName: "Alice", WhiteName: "Bob", Result: "B+7.5"})

	parsed, err := ParseSGF(sgf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.BoardSize != 9 {
		t.Errorf("board size = %d, want 9", parsed.BoardSize)
	}
	if parsed.BlackName != "Alice" || parsed.WhiteName != "Bob" {
		t.Errorf("names = %q/%q", parsed.BlackName, parsed.WhiteName)
	}
	if parsed.Winner() != "black" {
		t.Errorf("winner = %q, want black", parsed.Winner())
	}
	if len(parsed.Moves) != 3 {
		t.Fatalf("moves = %d, want 3", len(parsed.Moves))
	}
	// Replaying the parsed moves must reproduce the same position.
	replay := NewGame(parsed.Config())
	for i, m := range parsed.Moves {
		next, _, err := replay.Apply(m)
		if err != nil {
			t.Fatalf("replay move %d: %v", i, err)
		}
		replay = next
	}
	if replay.StateHash() != s.StateHash() {
		t.Error("replayed position does not match the exported one")
	}
}

func TestParseSGFHandlesPassesAndEscapes(t *testing.T) {
	g, err := ParseSGF(`(;FF[4]SZ[19]PB[A\]B]KM[7.5];B[dd];W[];B[tt])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if g.BlackName != "A]B" {
		t.Errorf("escaped name = %q, want %q", g.BlackName, "A]B")
	}
	if len(g.Moves) != 3 {
		t.Fatalf("moves = %d, want 3", len(g.Moves))
	}
	if g.Moves[1].Kind != Pass {
		t.Error("empty value should parse as a pass")
	}
	if g.Moves[2].Kind != Pass {
		t.Error("tt on a 19x19 board should parse as a pass")
	}
}

func TestParseSGFSkipsVariations(t *testing.T) {
	// The main line is dd, pp. The parenthesised branch must be ignored.
	g, err := ParseSGF(`(;FF[4]SZ[19];B[dd];W[pp](;B[qq])(;B[cc]))`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(g.Moves) != 2 {
		t.Errorf("moves = %d, want 2 (variations excluded)", len(g.Moves))
	}
}

func TestParseSGFReadsHandicapSetup(t *testing.T) {
	g, err := ParseSGF(`(;FF[4]SZ[19]HA[2]KM[0.5]AB[dd][pp];W[qf])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if g.Handicap != 2 {
		t.Errorf("handicap = %d, want 2", g.Handicap)
	}
	if len(g.SetupBlack) != 2 {
		t.Errorf("setup stones = %d, want 2", len(g.SetupBlack))
	}
	if g.Komi != 0.5 {
		t.Errorf("komi = %v, want 0.5", g.Komi)
	}
}

func TestParseSGFDefaultsUnknownRulesetToJapanese(t *testing.T) {
	g, err := ParseSGF(`(;FF[4]SZ[19]RU[SomethingElse];B[dd])`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := g.Config().Ruleset; got != Japanese {
		t.Errorf("ruleset = %s, want japanese for an unrecognised RU", got)
	}
}

func TestExportSGFEscapesBrackets(t *testing.T) {
	s := NewGame(NewConfig(19, Chinese, 0))
	out := ExportSGF(s, SGFMeta{BlackName: "A]B"})
	if !contains(out, `PB[A\]B]`) {
		t.Errorf("bracket not escaped in %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
