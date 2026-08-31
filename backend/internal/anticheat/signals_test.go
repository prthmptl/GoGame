package anticheat

import (
	"math"
	"testing"
)

func TestEngineMatchIgnoresShortGames(t *testing.T) {
	// 10 of 10 matches looks damning but proves nothing on a tiny sample.
	s := EngineMatchRate(10, 10)
	if s.Score != 0 {
		t.Errorf("score = %.2f, want 0 for a sample this small", s.Score)
	}
	if s.Detail["reason"] != "too_few_moves" {
		t.Errorf("detail should explain the zero: %v", s.Detail)
	}
}

func TestEngineMatchScoresOnlyTheExcessOverBaseline(t *testing.T) {
	// A strong human matching the baseline should not be flagged at all.
	if s := EngineMatchRate(45, 100); s.Score != 0 {
		t.Errorf("baseline match rate scored %.2f, want 0", s.Score)
	}
	// Near-perfect correlation should score close to 1.
	s := EngineMatchRate(98, 100)
	if s.Score < 0.9 {
		t.Errorf("98%% engine match scored %.2f, want > 0.9", s.Score)
	}
	// And it must be monotonic.
	low := EngineMatchRate(60, 100).Score
	high := EngineMatchRate(80, 100).Score
	if high <= low {
		t.Errorf("80%% scored %.2f but 60%% scored %.2f", high, low)
	}
}

func TestMoveTimingFlagsMechanicalUniformity(t *testing.T) {
	// A bot relaying moves: every move takes almost exactly 3 seconds.
	uniform := make([]int, 40)
	for i := range uniform {
		uniform[i] = 3000 + (i%3)*10
	}
	s := MoveTimingUniformity(uniform)
	if s.Score < 0.8 {
		t.Errorf("near-constant timings scored %.2f, want high", s.Score)
	}
}

func TestMoveTimingAcceptsHumanVariance(t *testing.T) {
	// A human: mostly quick, occasionally a long think.
	human := []int{1200, 800, 15000, 2300, 600, 40000, 1100, 900, 3000, 700,
		25000, 1500, 800, 1200, 60000, 900, 1100, 2000, 700, 1300}
	s := MoveTimingUniformity(human)
	if s.Score > 0.1 {
		t.Errorf("human timing scored %.2f, want near 0", s.Score)
	}
}

func TestRatingJumpNeedsASample(t *testing.T) {
	if s := RatingJump(400, 5); s.Score != 0 {
		t.Errorf("score = %.2f, want 0 over only 5 games", s.Score)
	}
	// 30 points a game over 40 games is extreme.
	if s := RatingJump(1200, 40); s.Score < 0.9 {
		t.Errorf("score = %.2f, want high for a 30/game climb", s.Score)
	}
	// Normal improvement should be quiet.
	if s := RatingJump(100, 50); s.Score > 0.15 {
		t.Errorf("score = %.2f, want low for ordinary improvement", s.Score)
	}
}

func TestAccuracySplitFlagsRankedOnlySharpness(t *testing.T) {
	s := AccuracySplit(0.92, 0.62, 20, 20)
	if s.Score < 0.9 {
		t.Errorf("a 30-point gap scored %.2f, want high", s.Score)
	}
	if s := AccuracySplit(0.72, 0.70, 20, 20); s.Score > 0.2 {
		t.Errorf("a 2-point gap scored %.2f, want low", s.Score)
	}
}

func TestLosingDisconnectNeedsAnExcessOverChance(t *testing.T) {
	// Half while losing is what chance produces.
	if s := LosingDisconnect(5, 10); s.Score != 0 {
		t.Errorf("score = %.2f, want 0 at the chance rate", s.Score)
	}
	if s := LosingDisconnect(20, 20); s.Score < 0.9 {
		t.Errorf("always disconnecting while losing scored %.2f, want high", s.Score)
	}
}

func TestCompositeRenormalisesOverPresentSignals(t *testing.T) {
	// One maxed signal on its own should produce a high composite, not one
	// scaled down by four absent signals.
	only := []Signal{{Kind: KindEngineMatch, Score: 1.0}}
	if got := Composite(only); math.Abs(got-1.0) > 0.001 {
		t.Errorf("composite = %.3f, want 1.0 when the only signal is maxed", got)
	}
	// A clean player scores zero.
	clean := []Signal{
		{Kind: KindEngineMatch, Score: 0}, {Kind: KindMoveTiming, Score: 0},
	}
	if got := Composite(clean); got != 0 {
		t.Errorf("composite = %.3f, want 0", got)
	}
}

func TestCompositeWeightsEngineMatchHighest(t *testing.T) {
	engine := Composite([]Signal{
		{Kind: KindEngineMatch, Score: 1}, {Kind: KindLosingDisconnect, Score: 0},
	})
	disconnect := Composite([]Signal{
		{Kind: KindEngineMatch, Score: 0}, {Kind: KindLosingDisconnect, Score: 1},
	})
	if engine <= disconnect {
		t.Errorf("engine-match composite %.3f should exceed disconnect %.3f", engine, disconnect)
	}
}

func TestShouldFlagRequiresBreadthNotJustHeight(t *testing.T) {
	// One maxed signal: composite is 1.0 but it is a single data point.
	single := []Signal{{Kind: KindEngineMatch, Score: 1.0}}
	if ShouldFlag(single, Composite(single)) {
		t.Error("a single signal opened a case; a reviewer needs corroboration")
	}
	// Three corroborating signals should flag.
	several := []Signal{
		{Kind: KindEngineMatch, Score: 0.9},
		{Kind: KindMoveTiming, Score: 0.8},
		{Kind: KindRatingJump, Score: 0.7},
	}
	if !ShouldFlag(several, Composite(several)) {
		t.Errorf("three strong signals (composite %.2f) did not flag", Composite(several))
	}
	// A clean player never flags.
	cleanSet := []Signal{
		{Kind: KindEngineMatch, Score: 0.1}, {Kind: KindMoveTiming, Score: 0.05},
		{Kind: KindRatingJump, Score: 0.1},
	}
	if ShouldFlag(cleanSet, Composite(cleanSet)) {
		t.Error("a clean player was flagged")
	}
}

func TestTopSignalsOrdersByContribution(t *testing.T) {
	in := []Signal{
		{Kind: KindLosingDisconnect, Score: 1.0}, // weight 0.10 -> 0.10
		{Kind: KindEngineMatch, Score: 0.5},      // weight 0.40 -> 0.20
	}
	out := TopSignals(in)
	if out[0].Kind != KindEngineMatch {
		t.Errorf("first signal = %s, want the highest-contribution one", out[0].Kind)
	}
}
