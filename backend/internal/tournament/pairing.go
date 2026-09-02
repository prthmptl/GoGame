// Package tournament implements E3: arena, Swiss/McMahon and knockout events.
package tournament

import (
	"sort"

	"github.com/google/uuid"
)

// Entrant is one competitor's standing.
type Entrant struct {
	UserID       uuid.UUID
	SeedRating   int
	Score        float64
	Tiebreak     float64
	Withdrawn    bool
	EliminatedIn *int
	// Opponents already met, so Swiss and McMahon never repeat a pairing.
	Opponents map[uuid.UUID]bool
	// HadBye stops one player receiving two byes while others receive none.
	HadBye bool
}

// Pairing is one board in a round.
type Pairing struct {
	Black uuid.UUID
	White uuid.UUID
	IsBye bool
}

// SwissPairings builds one round of Swiss (or McMahon — the algorithm is the
// same once starting scores are seeded).
//
// Players are sorted into score groups and paired within them, strongest
// against strongest. A player who cannot be paired inside their group floats
// down to the next, which is what keeps the field pairable in later rounds.
func SwissPairings(entrants []Entrant) []Pairing {
	active := make([]Entrant, 0, len(entrants))
	for _, e := range entrants {
		if !e.Withdrawn {
			active = append(active, e)
		}
	}
	// Sort by score, then rating: the standard Swiss ordering.
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].Score != active[j].Score {
			return active[i].Score > active[j].Score
		}
		return active[i].SeedRating > active[j].SeedRating
	})

	var pairings []Pairing
	paired := make(map[uuid.UUID]bool, len(active))

	// An odd field means someone gets a bye. Give it to the lowest-scoring
	// player who has not had one, which is the conventional rule.
	if len(active)%2 == 1 {
		for i := len(active) - 1; i >= 0; i-- {
			if !active[i].HadBye {
				pairings = append(pairings, Pairing{Black: active[i].UserID, IsBye: true})
				paired[active[i].UserID] = true
				break
			}
		}
		// Everyone has had a bye already: give it to the lowest scorer.
		if len(pairings) == 0 && len(active) > 0 {
			last := active[len(active)-1]
			pairings = append(pairings, Pairing{Black: last.UserID, IsBye: true})
			paired[last.UserID] = true
		}
	}

	for i := range active {
		a := active[i]
		if paired[a.UserID] {
			continue
		}
		// Find the nearest player in the ordering who is neither paired nor a
		// previous opponent.
		partner := -1
		for j := i + 1; j < len(active); j++ {
			b := active[j]
			if paired[b.UserID] || a.Opponents[b.UserID] {
				continue
			}
			partner = j
			break
		}
		if partner < 0 {
			// Nobody left they have not played. Accept a repeat rather than
			// leaving them unpaired — an unplayed round is worse.
			for j := i + 1; j < len(active); j++ {
				if !paired[active[j].UserID] {
					partner = j
					break
				}
			}
		}
		if partner < 0 {
			continue
		}
		b := active[partner]
		paired[a.UserID], paired[b.UserID] = true, true
		// Alternate colours by who has had fewer blacks; approximated here by
		// seed so the higher seed does not always take black.
		if a.SeedRating >= b.SeedRating {
			pairings = append(pairings, Pairing{Black: b.UserID, White: a.UserID})
		} else {
			pairings = append(pairings, Pairing{Black: a.UserID, White: b.UserID})
		}
	}
	return pairings
}

// McMahonSeed converts a rating into a starting score.
//
// McMahon exists so a 5-dan does not spend three rounds beating beginners:
// stronger players start on a higher score and meet each other immediately.
// Everyone at or above the bar starts level, which is the "top group".
func McMahonSeed(rating, barRating int, groupSize int) float64 {
	if groupSize <= 0 {
		groupSize = 100
	}
	if rating >= barRating {
		return 0
	}
	// Each group below the bar starts one point lower.
	below := (barRating - rating + groupSize - 1) / groupSize
	return -float64(below)
}

// KnockoutPairings builds one round of single elimination from the survivors,
// pairing the highest seed against the lowest.
func KnockoutPairings(entrants []Entrant) []Pairing {
	alive := make([]Entrant, 0, len(entrants))
	for _, e := range entrants {
		if !e.Withdrawn && e.EliminatedIn == nil {
			alive = append(alive, e)
		}
	}
	sort.SliceStable(alive, func(i, j int) bool {
		return alive[i].SeedRating > alive[j].SeedRating
	})

	var pairings []Pairing
	// An odd survivor count gives the top seed a bye into the next round.
	if len(alive)%2 == 1 {
		pairings = append(pairings, Pairing{Black: alive[0].UserID, IsBye: true})
		alive = alive[1:]
	}
	for i, j := 0, len(alive)-1; i < j; i, j = i+1, j-1 {
		pairings = append(pairings, Pairing{Black: alive[j].UserID, White: alive[i].UserID})
	}
	return pairings
}

// ArenaPairings pairs whoever is currently free, closest in score first.
//
// Arena has no rounds: a player finishing a game goes straight back into the
// pool, so this is called continuously with whoever is idle.
func ArenaPairings(idle []Entrant) []Pairing {
	available := make([]Entrant, 0, len(idle))
	for _, e := range idle {
		if !e.Withdrawn {
			available = append(available, e)
		}
	}
	sort.SliceStable(available, func(i, j int) bool {
		if available[i].Score != available[j].Score {
			return available[i].Score > available[j].Score
		}
		return available[i].SeedRating > available[j].SeedRating
	})

	var pairings []Pairing
	used := make([]bool, len(available))
	for i := range available {
		if used[i] {
			continue
		}
		// Prefer someone they have not just played, but in arena a rematch is
		// acceptable rather than sitting a player out.
		best := -1
		for j := i + 1; j < len(available); j++ {
			if used[j] {
				continue
			}
			if available[i].Opponents[available[j].UserID] && best < 0 {
				best = j
				continue
			}
			if !available[i].Opponents[available[j].UserID] {
				best = j
				break
			}
		}
		if best < 0 {
			continue
		}
		used[i], used[best] = true, true
		if available[i].SeedRating >= available[best].SeedRating {
			pairings = append(pairings, Pairing{Black: available[best].UserID, White: available[i].UserID})
		} else {
			pairings = append(pairings, Pairing{Black: available[i].UserID, White: available[best].UserID})
		}
	}
	return pairings
}

// ComputeTiebreaks fills in each entrant's tiebreak as the sum of their
// opponents' scores (Solkoff / SOS), the standard Go tournament tiebreak.
func ComputeTiebreaks(entrants []Entrant) []Entrant {
	scores := make(map[uuid.UUID]float64, len(entrants))
	for _, e := range entrants {
		scores[e.UserID] = e.Score
	}
	out := make([]Entrant, len(entrants))
	copy(out, entrants)
	for i := range out {
		var sos float64
		for opp := range out[i].Opponents {
			sos += scores[opp]
		}
		out[i].Tiebreak = sos
	}
	return out
}

// Standings orders entrants for display: score, then tiebreak, then rating.
func Standings(entrants []Entrant) []Entrant {
	out := ComputeTiebreaks(entrants)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Tiebreak != out[j].Tiebreak {
			return out[i].Tiebreak > out[j].Tiebreak
		}
		return out[i].SeedRating > out[j].SeedRating
	})
	return out
}
