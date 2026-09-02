// Package rating implements E1: Glicko-2 ratings.
//
// Follows Mark Glickman's published algorithm
// (http://www.glicko.net/glicko/glicko2.pdf). Ratings are stored on the
// familiar Glicko scale (1500 ± 350) and converted to the internal scale for
// the update, which is what the paper's step numbering refers to.
package rating

import "math"

// Tunables.
const (
	// DefaultRating is the starting point for a new player.
	DefaultRating = 1500.0
	// DefaultDeviation is maximal uncertainty: a brand-new player.
	DefaultDeviation = 350.0
	// DefaultVolatility is Glickman's suggested starting value.
	DefaultVolatility = 0.06

	// scale converts between the Glicko and Glicko-2 scales.
	scale = 173.7178

	// tau constrains how much volatility may change per rating period.
	// Smaller values damp swings; 0.5 is the paper's suggested range for
	// games where results are not very erratic.
	tau = 0.5

	// convergence is the iteration tolerance for the volatility solver.
	convergence = 0.000001

	// maxDeviation caps deviation so an inactive player never becomes
	// completely unranked.
	maxDeviation = 350.0
	// minDeviation floors deviation so an established rating stays
	// responsive to a genuine change in strength.
	minDeviation = 30.0
)

// Player is one competitor's rating state.
type Player struct {
	Rating     float64
	Deviation  float64
	Volatility float64
}

// NewPlayer returns an unrated player.
func NewPlayer() Player {
	return Player{Rating: DefaultRating, Deviation: DefaultDeviation, Volatility: DefaultVolatility}
}

// Outcome is a result from the perspective of the player being updated.
type Outcome float64

// The three possible scores.
const (
	Loss Outcome = 0.0
	Draw Outcome = 0.5
	Win  Outcome = 1.0
)

// Update returns p's new rating after a single game against opponent.
//
// Glicko-2 is defined over a rating *period* containing several games. Online
// play needs an update per game, so each game is treated as a period of one —
// the standard adaptation, and what every online Go and chess server does.
func Update(p, opponent Player, score Outcome) Player {
	// Step 2: convert to the Glicko-2 scale.
	mu := (p.Rating - DefaultRating) / scale
	phi := p.Deviation / scale
	muOpp := (opponent.Rating - DefaultRating) / scale
	phiOpp := opponent.Deviation / scale

	// Step 3: the variance of the expected outcome.
	g := gFunc(phiOpp)
	e := eFunc(mu, muOpp, phiOpp)
	v := 1.0 / (g * g * e * (1 - e))

	// Step 4: the estimated improvement.
	delta := v * g * (float64(score) - e)

	// Step 5: solve for the new volatility.
	sigmaPrime := newVolatility(phi, v, delta, p.Volatility)

	// Step 6: pre-rating-period deviation.
	phiStar := math.Sqrt(phi*phi + sigmaPrime*sigmaPrime)

	// Step 7: the new deviation and rating.
	phiPrime := 1.0 / math.Sqrt(1.0/(phiStar*phiStar)+1.0/v)
	muPrime := mu + phiPrime*phiPrime*g*(float64(score)-e)

	// Step 8: back to the Glicko scale.
	out := Player{
		Rating:     muPrime*scale + DefaultRating,
		Deviation:  phiPrime * scale,
		Volatility: sigmaPrime,
	}
	out.Deviation = math.Min(maxDeviation, math.Max(minDeviation, out.Deviation))
	return out
}

// gFunc dampens the effect of a game against an uncertain opponent.
func gFunc(phi float64) float64 {
	return 1.0 / math.Sqrt(1.0+3.0*phi*phi/(math.Pi*math.Pi))
}

// eFunc is the expected score.
func eFunc(mu, muOpp, phiOpp float64) float64 {
	return 1.0 / (1.0 + math.Exp(-gFunc(phiOpp)*(mu-muOpp)))
}

// newVolatility runs the paper's illinois-variant regula falsi iteration.
func newVolatility(phi, v, delta, sigma float64) float64 {
	a := math.Log(sigma * sigma)
	f := func(x float64) float64 {
		ex := math.Exp(x)
		num := ex * (delta*delta - phi*phi - v - ex)
		den := 2.0 * (phi*phi + v + ex) * (phi*phi + v + ex)
		return num/den - (x-a)/(tau*tau)
	}

	// Bracket the root.
	A := a
	var B float64
	if delta*delta > phi*phi+v {
		B = math.Log(delta*delta - phi*phi - v)
	} else {
		k := 1.0
		for f(a-k*tau) < 0 {
			k++
			if k > 100 {
				// Should never happen; bail out rather than spin.
				return sigma
			}
		}
		B = a - k*tau
	}

	fA, fB := f(A), f(B)
	for i := 0; math.Abs(B-A) > convergence && i < 100; i++ {
		C := A + (A-B)*fA/(fB-fA)
		fC := f(C)
		if fC*fB <= 0 {
			A, fA = B, fB
		} else {
			fA /= 2.0
		}
		B, fB = C, fC
	}
	return math.Exp(A / 2.0)
}

// DecayForInactivity inflates deviation for a player who has not played,
// so a long-dormant rating becomes uncertain again rather than staying
// artificially precise. periods is the number of rating periods missed.
func DecayForInactivity(p Player, periods float64) Player {
	if periods <= 0 {
		return p
	}
	phi := p.Deviation / scale
	phiStar := math.Sqrt(phi*phi + periods*p.Volatility*p.Volatility)
	p.Deviation = math.Min(maxDeviation, phiStar*scale)
	return p
}

// ConservativeRating is the rating to display and rank on: the bottom of the
// 95% confidence interval. A new player with a huge deviation therefore does
// not appear at the top of a leaderboard on one lucky win.
func ConservativeRating(p Player) int {
	return int(math.Round(p.Rating - 2*p.Deviation))
}
