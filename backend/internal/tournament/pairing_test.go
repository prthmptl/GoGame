package tournament

import (
	"testing"

	"github.com/google/uuid"
)

func mk(rating int, score float64, opponents ...uuid.UUID) Entrant {
	e := Entrant{UserID: uuid.New(), SeedRating: rating, Score: score,
		Opponents: map[uuid.UUID]bool{}}
	for _, o := range opponents {
		e.Opponents[o] = true
	}
	return e
}

func allPaired(t *testing.T, entrants []Entrant, pairings []Pairing) {
	t.Helper()
	seen := map[uuid.UUID]int{}
	for _, p := range pairings {
		seen[p.Black]++
		if !p.IsBye {
			seen[p.White]++
		}
	}
	for _, e := range entrants {
		if e.Withdrawn {
			continue
		}
		if seen[e.UserID] != 1 {
			t.Errorf("player %s appears in %d pairings, want exactly 1", e.UserID, seen[e.UserID])
		}
	}
}

func TestSwissPairsEveryoneExactlyOnce(t *testing.T) {
	entrants := []Entrant{mk(2000, 2), mk(1900, 2), mk(1800, 1), mk(1700, 1),
		mk(1600, 0), mk(1500, 0)}
	pairings := SwissPairings(entrants)
	if len(pairings) != 3 {
		t.Fatalf("pairings = %d, want 3 boards for 6 players", len(pairings))
	}
	allPaired(t, entrants, pairings)
}

func TestSwissPairsWithinScoreGroups(t *testing.T) {
	top1, top2 := mk(2000, 3), mk(1500, 3)
	bot1, bot2 := mk(1900, 0), mk(1400, 0)
	entrants := []Entrant{top1, bot1, top2, bot2}

	pairings := SwissPairings(entrants)
	if len(pairings) != 2 {
		t.Fatalf("pairings = %d, want 2", len(pairings))
	}
	// The two 3-pointers should meet each other, not the 0-pointers, even
	// though ratings cut across the groups.
	tops := map[uuid.UUID]bool{top1.UserID: true, top2.UserID: true}
	for _, p := range pairings {
		if tops[p.Black] != tops[p.White] {
			t.Errorf("pairing crossed score groups: %v vs %v", p.Black, p.White)
		}
	}
}

func TestSwissNeverRepeatsAPairingWhenAvoidable(t *testing.T) {
	a := mk(2000, 1)
	b := mk(1900, 1)
	c := mk(1800, 1)
	d := mk(1700, 1)
	// a and b have already played each other.
	a.Opponents[b.UserID] = true
	b.Opponents[a.UserID] = true

	pairings := SwissPairings([]Entrant{a, b, c, d})
	for _, p := range pairings {
		if (p.Black == a.UserID && p.White == b.UserID) ||
			(p.Black == b.UserID && p.White == a.UserID) {
			t.Error("Swiss repeated a pairing that could have been avoided")
		}
	}
	allPaired(t, []Entrant{a, b, c, d}, pairings)
}

func TestSwissGivesTheByeToTheLowestScorerWithoutOne(t *testing.T) {
	strong := mk(2000, 3)
	middle := mk(1800, 2)
	weak := mk(1500, 0)
	pairings := SwissPairings([]Entrant{strong, middle, weak})

	var byes int
	for _, p := range pairings {
		if p.IsBye {
			byes++
			if p.Black != weak.UserID {
				t.Errorf("bye went to %v, want the lowest scorer", p.Black)
			}
		}
	}
	if byes != 1 {
		t.Errorf("byes = %d, want exactly 1 in an odd field", byes)
	}
}

func TestSwissDoesNotGiveASecondByeWhileOthersHaveNone(t *testing.T) {
	a := mk(2000, 2)
	b := mk(1800, 1)
	c := mk(1500, 0)
	c.HadBye = true // the lowest scorer already sat out once

	pairings := SwissPairings([]Entrant{a, b, c})
	for _, p := range pairings {
		if p.IsBye && p.Black == c.UserID {
			t.Error("the same player received a second bye while others had none")
		}
	}
}

func TestSwissSkipsWithdrawnPlayers(t *testing.T) {
	a, b := mk(2000, 1), mk(1900, 1)
	gone := mk(1800, 1)
	gone.Withdrawn = true

	pairings := SwissPairings([]Entrant{a, b, gone})
	for _, p := range pairings {
		if p.Black == gone.UserID || p.White == gone.UserID {
			t.Error("a withdrawn player was paired")
		}
	}
	if len(pairings) != 1 {
		t.Errorf("pairings = %d, want 1 board from the two remaining players", len(pairings))
	}
}

func TestMcMahonSeedsStrongPlayersTogether(t *testing.T) {
	// Everyone at or above the bar starts level.
	if got := McMahonSeed(2200, 2000, 100); got != 0 {
		t.Errorf("seed for a player above the bar = %v, want 0", got)
	}
	if got := McMahonSeed(2000, 2000, 100); got != 0 {
		t.Errorf("seed at the bar = %v, want 0", got)
	}
	// Each group below the bar starts a point lower.
	if got := McMahonSeed(1900, 2000, 100); got != -1 {
		t.Errorf("seed one group below = %v, want -1", got)
	}
	if got := McMahonSeed(1750, 2000, 100); got != -3 {
		t.Errorf("seed for 1750 = %v, want -3", got)
	}
	// Weaker players are further down, so they meet each other first.
	if McMahonSeed(1200, 2000, 100) >= McMahonSeed(1800, 2000, 100) {
		t.Error("a weaker player should seed below a stronger one")
	}
}

func TestKnockoutPairsTopSeedAgainstBottom(t *testing.T) {
	a, b, c, d := mk(2000, 0), mk(1900, 0), mk(1800, 0), mk(1700, 0)
	pairings := KnockoutPairings([]Entrant{a, b, c, d})
	if len(pairings) != 2 {
		t.Fatalf("pairings = %d, want 2", len(pairings))
	}
	// 1v4 and 2v3.
	found := map[string]bool{}
	for _, p := range pairings {
		key := ""
		for _, id := range []uuid.UUID{p.Black, p.White} {
			switch id {
			case a.UserID:
				key += "a"
			case b.UserID:
				key += "b"
			case c.UserID:
				key += "c"
			case d.UserID:
				key += "d"
			}
		}
		found[sortString(key)] = true
	}
	if !found["ad"] || !found["bc"] {
		t.Errorf("pairings = %v, want top-vs-bottom (a-d and b-c)", found)
	}
}

func sortString(s string) string {
	b := []byte(s)
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j] < b[j-1]; j-- {
			b[j], b[j-1] = b[j-1], b[j]
		}
	}
	return string(b)
}

func TestKnockoutExcludesEliminatedPlayers(t *testing.T) {
	a, b := mk(2000, 0), mk(1900, 0)
	out := mk(1800, 0)
	round := 1
	out.EliminatedIn = &round

	pairings := KnockoutPairings([]Entrant{a, b, out})
	if len(pairings) != 1 {
		t.Fatalf("pairings = %d, want 1 from the two survivors", len(pairings))
	}
	for _, p := range pairings {
		if p.Black == out.UserID || p.White == out.UserID {
			t.Error("an eliminated player was paired")
		}
	}
}

func TestKnockoutGivesTheTopSeedTheBye(t *testing.T) {
	a, b, c := mk(2100, 0), mk(1900, 0), mk(1700, 0)
	pairings := KnockoutPairings([]Entrant{a, b, c})
	var bye *Pairing
	for i := range pairings {
		if pairings[i].IsBye {
			bye = &pairings[i]
		}
	}
	if bye == nil {
		t.Fatal("no bye in an odd knockout field")
	}
	if bye.Black != a.UserID {
		t.Errorf("bye went to %v, want the top seed", bye.Black)
	}
}

func TestArenaPrefersUnplayedOpponents(t *testing.T) {
	a := mk(1800, 2)
	b := mk(1790, 2)
	c := mk(1780, 2)
	d := mk(1770, 2)
	// a has already played b.
	a.Opponents[b.UserID] = true
	b.Opponents[a.UserID] = true

	pairings := ArenaPairings([]Entrant{a, b, c, d})
	for _, p := range pairings {
		if (p.Black == a.UserID && p.White == b.UserID) ||
			(p.Black == b.UserID && p.White == a.UserID) {
			t.Error("arena rematched two players while fresh opponents were free")
		}
	}
}

func TestTiebreakIsSumOfOpponentScores(t *testing.T) {
	a := mk(1800, 2)
	b := mk(1700, 3)
	c := mk(1600, 1)
	a.Opponents[b.UserID] = true
	a.Opponents[c.UserID] = true

	out := ComputeTiebreaks([]Entrant{a, b, c})
	for _, e := range out {
		if e.UserID == a.UserID && e.Tiebreak != 4 {
			t.Errorf("tiebreak = %v, want 4 (3 + 1 from opponents)", e.Tiebreak)
		}
	}
}

func TestStandingsOrderByScoreThenTiebreak(t *testing.T) {
	strongOpp := mk(1900, 3)
	weakOpp := mk(1500, 0)
	// Two players on the same score; one beat a stronger field.
	tough := mk(1800, 2)
	easy := mk(1800, 2)
	tough.Opponents[strongOpp.UserID] = true
	easy.Opponents[weakOpp.UserID] = true

	standings := Standings([]Entrant{easy, tough, strongOpp, weakOpp})
	// strongOpp has 3 points and leads; among the 2-pointers, tough is ahead.
	var toughRank, easyRank int
	for i, e := range standings {
		if e.UserID == tough.UserID {
			toughRank = i
		}
		if e.UserID == easy.UserID {
			easyRank = i
		}
	}
	if toughRank >= easyRank {
		t.Errorf("tough ranked %d, easy ranked %d; the stronger field should win the tiebreak",
			toughRank, easyRank)
	}
}
