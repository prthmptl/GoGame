package rating

import (
	"math"
	"testing"
)

// TestGlickmanWorkedExample checks the implementation against the worked
// example in Glickman's paper (http://www.glicko.net/glicko/glicko2.pdf).
//
// The paper rates one player against three opponents in a single period; this
// implementation updates per game, so the check is on the intermediate
// quantities the paper prints, which are shared by both formulations.
func TestGlickmanWorkedExampleIntermediates(t *testing.T) {
	// Paper: player r=1500 RD=200, opponents 1400/30, 1550/100, 1700/300.
	phi := 200.0 / scale
	mu := (1500.0 - DefaultRating) / scale

	cases := []struct {
		oppRating, oppDev float64
		wantG, wantE      float64
	}{
		{1400, 30, 0.9955, 0.639},
		{1550, 100, 0.9531, 0.432},
		{1700, 300, 0.7242, 0.303},
	}
	for _, c := range cases {
		muOpp := (c.oppRating - DefaultRating) / scale
		phiOpp := c.oppDev / scale
		if g := gFunc(phiOpp); math.Abs(g-c.wantG) > 0.001 {
			t.Errorf("g(%v) = %.4f, want %.4f", c.oppDev, g, c.wantG)
		}
		if e := eFunc(mu, muOpp, phiOpp); math.Abs(e-c.wantE) > 0.001 {
			t.Errorf("E vs %v = %.3f, want %.3f", c.oppRating, e, c.wantE)
		}
	}
	_ = phi
}

func TestWinRaisesAndLossLowersRating(t *testing.T) {
	p := NewPlayer()
	opp := NewPlayer()

	won := Update(p, opp, Win)
	if won.Rating <= p.Rating {
		t.Errorf("rating after a win = %.1f, want above %.1f", won.Rating, p.Rating)
	}
	lost := Update(p, opp, Loss)
	if lost.Rating >= p.Rating {
		t.Errorf("rating after a loss = %.1f, want below %.1f", lost.Rating, p.Rating)
	}
	// Symmetric opponents mean symmetric movement.
	up := won.Rating - p.Rating
	down := p.Rating - lost.Rating
	if math.Abs(up-down) > 0.5 {
		t.Errorf("asymmetric update: +%.2f on a win, -%.2f on a loss", up, down)
	}
}

func TestDrawAgainstAnEqualIsNeutral(t *testing.T) {
	p := NewPlayer()
	drew := Update(p, NewPlayer(), Draw)
	if math.Abs(drew.Rating-p.Rating) > 0.5 {
		t.Errorf("rating moved %.2f on a draw between equals", drew.Rating-p.Rating)
	}
	// Uncertainty still falls: we learned something from the game.
	if drew.Deviation >= p.Deviation {
		t.Errorf("deviation = %.1f, want below the starting %.1f", drew.Deviation, p.Deviation)
	}
}

func TestDeviationFallsAsGamesArePlayed(t *testing.T) {
	p := NewPlayer()
	opp := NewPlayer()
	prev := p.Deviation
	for i := 0; i < 10; i++ {
		outcome := Win
		if i%2 == 1 {
			outcome = Loss
		}
		p = Update(p, opp, outcome)
		if p.Deviation > prev+0.001 {
			t.Fatalf("deviation rose from %.2f to %.2f at game %d", prev, p.Deviation, i)
		}
		prev = p.Deviation
	}
	if p.Deviation > 200 {
		t.Errorf("deviation after 10 games = %.1f, want well below the initial 350", p.Deviation)
	}
}

func TestBeatingAStrongerOpponentGainsMore(t *testing.T) {
	p := Player{Rating: 1500, Deviation: 200, Volatility: DefaultVolatility}
	weak := Player{Rating: 1200, Deviation: 50, Volatility: DefaultVolatility}
	strong := Player{Rating: 1800, Deviation: 50, Volatility: DefaultVolatility}

	vsWeak := Update(p, weak, Win).Rating - p.Rating
	vsStrong := Update(p, strong, Win).Rating - p.Rating
	if vsStrong <= vsWeak {
		t.Errorf("beating 1800 gained %.2f but beating 1200 gained %.2f", vsStrong, vsWeak)
	}
}

func TestUncertainPlayerMovesFaster(t *testing.T) {
	fresh := Player{Rating: 1500, Deviation: 350, Volatility: DefaultVolatility}
	settled := Player{Rating: 1500, Deviation: 50, Volatility: DefaultVolatility}
	opp := Player{Rating: 1500, Deviation: 50, Volatility: DefaultVolatility}

	freshGain := Update(fresh, opp, Win).Rating - fresh.Rating
	settledGain := Update(settled, opp, Win).Rating - settled.Rating
	if freshGain <= settledGain {
		t.Errorf("uncertain player gained %.2f, settled gained %.2f; want the uncertain one to move more",
			freshGain, settledGain)
	}
}

func TestDeviationIsBounded(t *testing.T) {
	p := Player{Rating: 1500, Deviation: 30, Volatility: DefaultVolatility}
	opp := Player{Rating: 1500, Deviation: 30, Volatility: DefaultVolatility}
	for i := 0; i < 200; i++ {
		p = Update(p, opp, Draw)
	}
	if p.Deviation < minDeviation-0.001 {
		t.Errorf("deviation = %.2f, want floored at %.0f", p.Deviation, minDeviation)
	}
}

func TestInactivityRaisesUncertainty(t *testing.T) {
	p := Player{Rating: 1600, Deviation: 50, Volatility: DefaultVolatility}
	decayed := DecayForInactivity(p, 20)
	if decayed.Deviation <= p.Deviation {
		t.Errorf("deviation = %.1f, want above %.1f after inactivity", decayed.Deviation, p.Deviation)
	}
	if decayed.Rating != p.Rating {
		t.Error("inactivity must not move the rating itself")
	}
	// Even after a long absence the player is no more uncertain than a
	// brand-new one.
	veryStale := DecayForInactivity(p, 100000)
	if veryStale.Deviation > maxDeviation+0.001 {
		t.Errorf("deviation = %.1f, want capped at %.0f", veryStale.Deviation, maxDeviation)
	}
}

func TestConservativeRatingPenalisesUncertainty(t *testing.T) {
	fresh := Player{Rating: 1800, Deviation: 350, Volatility: DefaultVolatility}
	settled := Player{Rating: 1700, Deviation: 40, Volatility: DefaultVolatility}
	// A newcomer on a hot streak must not outrank an established player.
	if ConservativeRating(fresh) >= ConservativeRating(settled) {
		t.Errorf("fresh %d should rank below settled %d",
			ConservativeRating(fresh), ConservativeRating(settled))
	}
}

func TestVolatilitySolverStaysStable(t *testing.T) {
	// A big upset is where the volatility iteration is most likely to
	// misbehave; the result must stay finite and sane.
	p := Player{Rating: 2400, Deviation: 40, Volatility: DefaultVolatility}
	weak := Player{Rating: 900, Deviation: 40, Volatility: DefaultVolatility}
	out := Update(p, weak, Loss)
	if math.IsNaN(out.Rating) || math.IsInf(out.Rating, 0) {
		t.Fatalf("rating is not finite: %v", out.Rating)
	}
	if math.IsNaN(out.Volatility) || out.Volatility <= 0 || out.Volatility > 1 {
		t.Errorf("volatility = %v, want a small positive number", out.Volatility)
	}
	if out.Rating >= p.Rating {
		t.Errorf("rating rose to %.1f after losing to a much weaker player", out.Rating)
	}
}
