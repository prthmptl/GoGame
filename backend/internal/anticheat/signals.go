// Package anticheat implements E2: signal collection, per-user scoring and
// the review queue.
//
// Nothing here decides that someone cheated. Every signal is circumstantial —
// a strong player legitimately plays engine-like moves, and a fast player
// legitimately moves fast. The output is a ranked queue for a human, and the
// only automatic consequence is that a case gets opened.
package anticheat

import (
	"math"
	"sort"
)

// Kind identifies a signal type. These are the five E2 lists.
type Kind string

const (
	// KindEngineMatch is how often the player's moves matched an engine's
	// preferred move.
	KindEngineMatch Kind = "engine_match"
	// KindMoveTiming is how unnaturally uniform their move times are.
	KindMoveTiming Kind = "move_timing"
	// KindRatingJump is how fast their rating climbed.
	KindRatingJump Kind = "rating_jump"
	// KindAccuracySplit is how differently they play by game type.
	KindAccuracySplit Kind = "accuracy_split"
	// KindLosingDisconnect is how often they vanish from losing positions.
	KindLosingDisconnect Kind = "losing_disconnect"
)

// weights decide each signal's contribution to the composite score. Engine
// correlation carries the most weight because it is the hardest to produce
// accidentally; disconnect patterns carry the least because connections drop
// for innocent reasons all the time.
var weights = map[Kind]float64{
	KindEngineMatch:      0.40,
	KindMoveTiming:       0.20,
	KindRatingJump:       0.15,
	KindAccuracySplit:    0.15,
	KindLosingDisconnect: 0.10,
}

// Thresholds for the review queue.
const (
	// SoftFlagThreshold opens a case for human review.
	SoftFlagThreshold = 0.65
	// MinSignalsToFlag stops a single noisy game from opening a case.
	MinSignalsToFlag = 3
)

// Signal is one observation about one player in one game.
type Signal struct {
	Kind   Kind           `json:"kind"`
	Score  float64        `json:"score"`
	Detail map[string]any `json:"detail"`
}

// EngineMatchRate scores how closely a player's moves tracked the engine's
// first choice.
//
// The baseline matters: strong players match a weak heuristic engine perhaps
// 40% of the time simply by both playing reasonable moves. Only the excess
// above that baseline is suspicious, and short games are discounted because
// a handful of matches proves nothing.
func EngineMatchRate(matched, total int) Signal {
	if total < 20 {
		// Too few moves to say anything; report nothing rather than noise.
		return Signal{Kind: KindEngineMatch, Score: 0,
			Detail: map[string]any{"matched": matched, "total": total, "reason": "too_few_moves"}}
	}
	rate := float64(matched) / float64(total)
	const baseline = 0.45
	excess := (rate - baseline) / (1 - baseline)
	return Signal{
		Kind:  KindEngineMatch,
		Score: clamp(excess),
		Detail: map[string]any{
			"matched": matched, "total": total, "rate": rate, "baseline": baseline,
		},
	}
}

// MoveTimingUniformity scores how mechanical a player's move times look.
//
// Humans think for wildly different lengths depending on the position: a
// reflex response takes a second, a life-and-death read takes a minute. A
// bot relaying engine moves tends to be far more uniform, so a low
// coefficient of variation is the signal.
func MoveTimingUniformity(thinkMillis []int) Signal {
	if len(thinkMillis) < 15 {
		return Signal{Kind: KindMoveTiming, Score: 0,
			Detail: map[string]any{"moves": len(thinkMillis), "reason": "too_few_moves"}}
	}
	var sum float64
	for _, m := range thinkMillis {
		sum += float64(m)
	}
	mean := sum / float64(len(thinkMillis))
	if mean <= 0 {
		return Signal{Kind: KindMoveTiming, Score: 0, Detail: map[string]any{"mean": mean}}
	}
	var variance float64
	for _, m := range thinkMillis {
		d := float64(m) - mean
		variance += d * d
	}
	variance /= float64(len(thinkMillis))
	cv := math.Sqrt(variance) / mean

	// Human play typically sits around cv 0.8-1.5. Below 0.35 is odd.
	const humanFloor = 0.35
	var score float64
	if cv < humanFloor {
		score = (humanFloor - cv) / humanFloor
	}
	return Signal{
		Kind: KindMoveTiming, Score: clamp(score),
		Detail: map[string]any{
			"meanMillis": mean, "coefficientOfVariation": cv, "moves": len(thinkMillis),
		},
	}
}

// RatingJump scores an implausibly fast climb.
func RatingJump(gained float64, games int) Signal {
	if games < 10 {
		return Signal{Kind: KindRatingJump, Score: 0,
			Detail: map[string]any{"games": games, "reason": "too_few_games"}}
	}
	perGame := gained / float64(games)
	// Sustained gains above ~15 points per game over a meaningful sample are
	// hard to produce by improvement alone.
	const suspicious = 15.0
	return Signal{
		Kind: KindRatingJump, Score: clamp(perGame / (suspicious * 2)),
		Detail: map[string]any{"gained": gained, "games": games, "perGame": perGame},
	}
}

// AccuracySplit scores a player who is far sharper in rated games than in
// casual ones — the signature of someone who only reaches for help when it
// counts.
func AccuracySplit(rankedAccuracy, casualAccuracy float64, rankedGames, casualGames int) Signal {
	if rankedGames < 5 || casualGames < 5 {
		return Signal{Kind: KindAccuracySplit, Score: 0,
			Detail: map[string]any{"reason": "too_few_games"}}
	}
	gap := rankedAccuracy - casualAccuracy
	// A 20-point accuracy gap is a lot; scale against that.
	return Signal{
		Kind: KindAccuracySplit, Score: clamp(gap / 0.20),
		Detail: map[string]any{
			"rankedAccuracy": rankedAccuracy, "casualAccuracy": casualAccuracy, "gap": gap,
		},
	}
}

// LosingDisconnect scores a player who disconnects mainly when losing, which
// is rage-quitting or timeout-farming rather than cheating, but belongs in
// the same review queue.
func LosingDisconnect(disconnectsWhileLosing, totalDisconnects int) Signal {
	if totalDisconnects < 5 {
		return Signal{Kind: KindLosingDisconnect, Score: 0,
			Detail: map[string]any{"total": totalDisconnects, "reason": "too_few"}}
	}
	rate := float64(disconnectsWhileLosing) / float64(totalDisconnects)
	// Half would be chance; the excess over that is the signal.
	return Signal{
		Kind: KindLosingDisconnect, Score: clamp((rate - 0.5) * 2),
		Detail: map[string]any{
			"whileLosing": disconnectsWhileLosing, "total": totalDisconnects, "rate": rate,
		},
	}
}

// Composite combines signals into one score.
//
// Weights are renormalised over the signals actually present, so a user with
// only two collected signals is not implicitly scored as though the other
// three came back clean.
func Composite(signals []Signal) float64 {
	var weighted, totalWeight float64
	for _, s := range signals {
		w, ok := weights[s.Kind]
		if !ok {
			continue
		}
		weighted += w * clamp(s.Score)
		totalWeight += w
	}
	if totalWeight == 0 {
		return 0
	}
	return weighted / totalWeight
}

// ShouldFlag decides whether a user warrants a case. Both conditions matter:
// a high score from one signal is not enough evidence to take up a
// reviewer's time.
func ShouldFlag(signals []Signal, composite float64) bool {
	meaningful := 0
	for _, s := range signals {
		if s.Score > 0 {
			meaningful++
		}
	}
	return meaningful >= MinSignalsToFlag && composite >= SoftFlagThreshold
}

// TopSignals returns the signals sorted by contribution, so a reviewer sees
// the strongest evidence first.
func TopSignals(signals []Signal) []Signal {
	out := make([]Signal, len(signals))
	copy(out, signals)
	sort.SliceStable(out, func(i, j int) bool {
		return weights[out[i].Kind]*out[i].Score > weights[out[j].Kind]*out[j].Score
	})
	return out
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
