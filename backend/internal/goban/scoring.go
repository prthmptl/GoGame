package goban

import "fmt"

// Score is the result of counting a finished position.
type Score struct {
	BlackStones    int           `json:"blackStones"`
	WhiteStones    int           `json:"whiteStones"`
	BlackTerritory int           `json:"blackTerritory"`
	WhiteTerritory int           `json:"whiteTerritory"`
	BlackPrisoners int           `json:"blackPrisoners"`
	WhitePrisoners int           `json:"whitePrisoners"`
	Neutral        int           `json:"neutral"`
	Komi           float64       `json:"komi"`
	Method         ScoringMethod `json:"method"`
}

// BlackTotal is black's final count under the configured method.
func (s Score) BlackTotal() float64 {
	if s.Method == AreaScoring {
		return float64(s.BlackStones + s.BlackTerritory)
	}
	return float64(s.BlackTerritory + s.BlackPrisoners)
}

// WhiteTotal is white's final count, komi included.
func (s Score) WhiteTotal() float64 {
	if s.Method == AreaScoring {
		return float64(s.WhiteStones+s.WhiteTerritory) + s.Komi
	}
	return float64(s.WhiteTerritory+s.WhitePrisoners) + s.Komi
}

// Margin is positive when black wins.
func (s Score) Margin() float64 { return s.BlackTotal() - s.WhiteTotal() }

// Result renders the SGF result string, e.g. "B+7.5".
func (s Score) Result() string {
	m := s.Margin()
	switch {
	case m > 0:
		return fmt.Sprintf("B+%.1f", m)
	case m < 0:
		return fmt.Sprintf("W+%.1f", -m)
	default:
		return "Draw"
	}
}

// Winner returns "black", "white" or "draw".
func (s Score) Winner() string {
	switch m := s.Margin(); {
	case m > 0:
		return "black"
	case m < 0:
		return "white"
	default:
		return "draw"
	}
}

// ScorePosition counts the position with the given stones removed as dead.
// Mirrors Scoring.score in the client's scoring.dart.
func ScorePosition(s *State, dead []Point) Score {
	method := s.Config.Scoring
	if method == "" {
		method = DefaultsFor(s.Config.Ruleset).ScoringMethod
	}

	// Dead stones become prisoners for the opponent under territory scoring.
	deadBlack, deadWhite := 0, 0
	for _, p := range dead {
		switch s.Board.At(p) {
		case Black:
			deadBlack++
		case White:
			deadWhite++
		}
	}

	board := s.Board
	if len(dead) > 0 {
		board = board.SetMany(dead, Empty)
	}
	size := board.Size()

	out := Score{Komi: s.Config.Komi, Method: method}
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			switch board.AtRC(r, c) {
			case Black:
				out.BlackStones++
			case White:
				out.WhiteStones++
			}
		}
	}

	// Flood-fill each empty region; a region touching only one colour is that
	// player's territory, anything touching both is neutral (dame).
	visited := make([]bool, size*size)
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			idx := board.Index(r, c)
			if visited[idx] {
				continue
			}
			if board.AtRC(r, c) != Empty {
				visited[idx] = true
				continue
			}
			region := 0
			touchesBlack, touchesWhite := false, false
			stack := []Point{{r, c}}
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pi := board.Index(p.Row, p.Col)
				if visited[pi] {
					continue
				}
				visited[pi] = true
				region++
				for _, n := range board.Neighbors(p) {
					switch board.At(n) {
					case Empty:
						if !visited[board.Index(n.Row, n.Col)] {
							stack = append(stack, n)
						}
					case Black:
						touchesBlack = true
					case White:
						touchesWhite = true
					}
				}
			}
			switch {
			case touchesBlack && !touchesWhite:
				out.BlackTerritory += region
			case touchesWhite && !touchesBlack:
				out.WhiteTerritory += region
			default:
				out.Neutral += region
			}
		}
	}

	out.BlackPrisoners = s.CapturesByBlack + deadWhite
	out.WhitePrisoners = s.CapturesByWhite + deadBlack
	return out
}
