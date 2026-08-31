package goban

import "testing"

func mustApply(t *testing.T, s *State, in Intent) *State {
	t.Helper()
	next, _, err := s.Apply(in)
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	return next
}

func expectReject(t *testing.T, s *State, in Intent, want Rejection) {
	t.Helper()
	_, _, err := s.Apply(in)
	if err == nil {
		t.Fatalf("expected rejection %s, move was accepted", want)
	}
	rej, ok := err.(*RejectedError)
	if !ok || rej.Reason != want {
		t.Fatalf("rejection = %v, want %s", err, want)
	}
}

func TestCaptureRemovesSurroundedStone(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	// Black surrounds the white stone at (1,1).
	s = mustApply(t, s, PlaceAt(Point{0, 1})) // B
	s = mustApply(t, s, PlaceAt(Point{1, 1})) // W
	s = mustApply(t, s, PlaceAt(Point{1, 0})) // B
	s = mustApply(t, s, PlaceAt(Point{5, 5})) // W elsewhere
	s = mustApply(t, s, PlaceAt(Point{1, 2})) // B
	s = mustApply(t, s, PlaceAt(Point{6, 6})) // W elsewhere
	s = mustApply(t, s, PlaceAt(Point{2, 1})) // B closes the last liberty

	if got := s.Board.At(Point{1, 1}); got != Empty {
		t.Errorf("captured stone still on board: %v", got)
	}
	if s.CapturesByBlack != 1 {
		t.Errorf("CapturesByBlack = %d, want 1", s.CapturesByBlack)
	}
}

func TestSuicideRejectedUnderChinese(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	// White walls off the corner; black playing (0,0) would have no liberty.
	s = mustApply(t, s, PlaceAt(Point{4, 4})) // B elsewhere
	s = mustApply(t, s, PlaceAt(Point{0, 1})) // W
	s = mustApply(t, s, PlaceAt(Point{4, 5})) // B elsewhere
	s = mustApply(t, s, PlaceAt(Point{1, 0})) // W
	expectReject(t, s, PlaceAt(Point{0, 0}), RejSuicide)
}

func TestSuicideAllowedUnderIngRemovesGroup(t *testing.T) {
	cfg := NewConfig(9, Ing, 0)
	if !cfg.AllowSuicide {
		t.Fatal("Ing should permit suicide")
	}
	s := NewGame(cfg)
	s = mustApply(t, s, PlaceAt(Point{4, 4}))
	s = mustApply(t, s, PlaceAt(Point{0, 1}))
	s = mustApply(t, s, PlaceAt(Point{4, 5}))
	s = mustApply(t, s, PlaceAt(Point{1, 0}))

	next := mustApply(t, s, PlaceAt(Point{0, 0}))
	if next.Board.At(Point{0, 0}) != Empty {
		t.Error("self-captured stone should have been removed")
	}
	// The suicided stone counts as a prisoner for the opponent.
	if next.CapturesByWhite != 1 {
		t.Errorf("CapturesByWhite = %d, want 1", next.CapturesByWhite)
	}
}

func TestSimpleKoBlocksImmediateRecapture(t *testing.T) {
	s := NewGame(NewConfig(9, Japanese, 0))
	// Build the classic ko shape:
	//      c0 c1 c2 c3
	//  r0   .  B  W  .
	//  r1   B  .  .  W
	//  r2   .  B  W  .
	for _, p := range []Point{
		{0, 1}, {0, 2}, // B, W
		{1, 0}, {1, 3}, // B, W
		{2, 1}, {2, 2}, // B, W
	} {
		s = mustApply(t, s, PlaceAt(p))
	}
	// Black fills (1,2); its only liberty is (1,1).
	s = mustApply(t, s, PlaceAt(Point{1, 2}))
	// White plays (1,1), capturing that single stone.
	s = mustApply(t, s, PlaceAt(Point{1, 1}))

	if s.Board.At(Point{1, 2}) != Empty {
		t.Fatal("white did not capture the black stone")
	}
	if s.KoPoint == nil || *s.KoPoint != (Point{1, 2}) {
		t.Fatalf("KoPoint = %v, want (1,2)", s.KoPoint)
	}
	// Black recapturing immediately is the ko violation.
	expectReject(t, s, PlaceAt(Point{1, 2}), RejKo)

	// After a ko threat elsewhere the ban lifts.
	s = mustApply(t, s, PlaceAt(Point{7, 7})) // B threat
	s = mustApply(t, s, PlaceAt(Point{8, 8})) // W answer
	if s.KoPoint != nil {
		t.Errorf("KoPoint = %v, want cleared after intervening moves", s.KoPoint)
	}
	if _, _, err := s.Apply(PlaceAt(Point{1, 2})); err != nil {
		t.Errorf("retaking the ko after a threat should be legal, got %v", err)
	}
}

func TestKoPointOnlySetForSingleStoneCapture(t *testing.T) {
	s := NewGame(NewConfig(9, Japanese, 0))
	// White plays two stones that black then captures together; a two-stone
	// capture must not arm the simple-ko ban.
	for _, p := range []Point{
		{0, 0}, {0, 1}, // B(0,0), W(0,1)
		{1, 1}, {0, 2}, // B(1,1), W(0,2)
		{1, 2}, {5, 5}, // B(1,2), W elsewhere
		{0, 3}, // B(0,3) captures white (0,1),(0,2)
	} {
		s = mustApply(t, s, PlaceAt(p))
	}
	if s.Board.At(Point{0, 1}) != Empty || s.Board.At(Point{0, 2}) != Empty {
		t.Fatal("expected a two-stone capture")
	}
	if s.CapturesByBlack != 2 {
		t.Fatalf("CapturesByBlack = %d, want 2", s.CapturesByBlack)
	}
	if s.KoPoint != nil {
		t.Errorf("KoPoint = %v, want nil after a multi-stone capture", s.KoPoint)
	}
}

func TestPositionalSuperkoRejectsRepeatedPosition(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	if s.Config.Superko != SuperkoPositional {
		t.Fatal("Chinese should use positional superko")
	}
	// Pre-seed the hash the position would take after black plays (4,4), as
	// if that position had already occurred earlier in the game.
	next, _, err := s.Apply(PlaceAt(Point{4, 4}))
	if err != nil {
		t.Fatalf("probe move: %v", err)
	}
	s.PreviousHashes[positionHash(s.Config.Superko, next.Board, White)] = struct{}{}

	expectReject(t, s, PlaceAt(Point{4, 4}), RejSuperko)
}

func TestSuperkoNoneAllowsRepeatedPosition(t *testing.T) {
	// Japanese enforces only the immediate ko, so a repeat that positional
	// superko would ban is legal here.
	s := NewGame(NewConfig(9, Japanese, 0))
	next, _, err := s.Apply(PlaceAt(Point{4, 4}))
	if err != nil {
		t.Fatalf("probe move: %v", err)
	}
	s.PreviousHashes[positionHash(SuperkoPositional, next.Board, White)] = struct{}{}

	if _, _, err := s.Apply(PlaceAt(Point{4, 4})); err != nil {
		t.Errorf("Japanese rules should not enforce superko, got %v", err)
	}
}

func TestTwoPassesEnterScoring(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	s = mustApply(t, s, PassIntent())
	if s.Status != StatusActive {
		t.Errorf("status after one pass = %s, want active", s.Status)
	}
	s = mustApply(t, s, PassIntent())
	if s.Status != StatusScoring {
		t.Errorf("status after two passes = %s, want scoring", s.Status)
	}
	// A move into a finished game must be refused.
	expectReject(t, s, PlaceAt(Point{0, 0}), RejGameNotActive)
}

func TestResignEndsGame(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	s = mustApply(t, s, ResignIntent())
	if s.Status != StatusResigned {
		t.Errorf("status = %s, want resigned", s.Status)
	}
}

func TestOccupiedAndOutOfBounds(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	s = mustApply(t, s, PlaceAt(Point{3, 3}))
	expectReject(t, s, PlaceAt(Point{3, 3}), RejOccupied)
	expectReject(t, s, PlaceAt(Point{9, 0}), RejOutOfBounds)
	expectReject(t, s, PlaceAt(Point{-1, 0}), RejOutOfBounds)
}

func TestHandicapPlacesStonesAndWhiteOpens(t *testing.T) {
	s := NewGame(NewConfig(19, Chinese, 4))
	if s.ToMove != White {
		t.Errorf("ToMove = %v, want White to open with handicap", s.ToMove)
	}
	count := 0
	for r := 0; r < 19; r++ {
		for c := 0; c < 19; c++ {
			if s.Board.AtRC(r, c) == Black {
				count++
			}
		}
	}
	if count != 4 {
		t.Errorf("handicap stones = %d, want 4", count)
	}
}

func TestApplyDoesNotMutateReceiver(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	before := s.StateHash()
	if _, _, err := s.Apply(PlaceAt(Point{4, 4})); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if s.StateHash() != before {
		t.Error("Apply mutated the receiver; states must be immutable")
	}
	if len(s.History) != 0 {
		t.Errorf("history grew on the receiver: %d", len(s.History))
	}
}

func TestStateHashIsStableAndSideSensitive(t *testing.T) {
	a := NewGame(NewConfig(9, Chinese, 0))
	b := NewGame(NewConfig(9, Chinese, 0))
	if a.StateHash() != b.StateHash() {
		t.Error("identical positions hashed differently")
	}
	// Same board, different side to move must differ: it is a different
	// position for the purposes of a reconnect snapshot.
	if a.Board.StateHash(Black) == a.Board.StateHash(White) {
		t.Error("state hash ignores side to move")
	}
}

func TestLegalPlacementsExcludesOccupiedAndSuicide(t *testing.T) {
	s := NewGame(NewConfig(9, Chinese, 0))
	s = mustApply(t, s, PlaceAt(Point{4, 4}))
	legal := s.LegalPlacements()
	if len(legal) != 80 {
		t.Errorf("legal placements = %d, want 80 on a 9x9 with one stone", len(legal))
	}
	for _, p := range legal {
		if p == (Point{4, 4}) {
			t.Error("occupied point reported as legal")
		}
	}
}

func TestFindGroupCollectsConnectedStonesAndLiberties(t *testing.T) {
	b := NewBoard(9)
	b = b.Set(Point{0, 0}, Black)
	b = b.Set(Point{0, 1}, Black)
	b = b.Set(Point{1, 0}, Black)
	g := FindGroup(b, Point{0, 0})
	if len(g.Stones) != 3 {
		t.Errorf("group size = %d, want 3", len(g.Stones))
	}
	// Liberties: (0,2), (1,1), (2,0)
	if len(g.Liberties) != 3 {
		t.Errorf("liberties = %d, want 3", len(g.Liberties))
	}
}

func TestRulesetDefaultsMatchClient(t *testing.T) {
	cases := []struct {
		r       Ruleset
		komi    float64
		suicide bool
		superko SuperkoMode
		scoring ScoringMethod
	}{
		{Chinese, 7.5, false, SuperkoPositional, AreaScoring},
		{Japanese, 6.5, false, SuperkoNone, TerritoryScoring},
		{Korean, 6.5, false, SuperkoNone, TerritoryScoring},
		{AGA, 7.5, false, SuperkoSituational, AreaScoring},
		{Ing, 8.0, true, SuperkoSituational, AreaScoring},
		{NewZealand, 7.0, true, SuperkoSituational, AreaScoring},
		{TrompTaylor, 7.5, true, SuperkoPositional, AreaScoring},
	}
	for _, c := range cases {
		d := DefaultsFor(c.r)
		if d.Komi != c.komi || d.AllowSuicide != c.suicide ||
			d.Superko != c.superko || d.ScoringMethod != c.scoring {
			t.Errorf("%s defaults = %+v, want komi=%v suicide=%v superko=%v scoring=%v",
				c.r, d, c.komi, c.suicide, c.superko, c.scoring)
		}
	}
}
