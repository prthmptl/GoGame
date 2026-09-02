package goban

import (
	"fmt"
	"strings"
	"time"
)

// SGFMeta carries the tags that are not derivable from the game state.
type SGFMeta struct {
	BlackName string
	WhiteName string
	BlackRank string
	WhiteRank string
	Result    string
	Date      time.Time
	Event     string
	TimeLimit int // seconds
}

// coord converts a point to SGF's letter pair. SGF is column-major from the
// top-left: "aa" is the top-left intersection.
func coord(p Point) string {
	return string(rune('a'+p.Col)) + string(rune('a'+p.Row))
}

// ExportSGF renders a finished or in-progress game as FF[4] SGF, matching the
// client's sgf.dart output so a game archived by the server opens in the app.
func ExportSGF(s *State, meta SGFMeta) string {
	var b strings.Builder
	b.WriteString("(;FF[4]GM[1]CA[UTF-8]AP[gogame:1]")
	fmt.Fprintf(&b, "SZ[%d]", s.Board.Size())
	fmt.Fprintf(&b, "KM[%.1f]", s.Config.Komi)
	if s.Config.Handicap > 0 {
		fmt.Fprintf(&b, "HA[%d]", s.Config.Handicap)
	}
	b.WriteString("RU[" + escapeSGF(string(s.Config.Ruleset)) + "]")
	if meta.BlackName != "" {
		b.WriteString("PB[" + escapeSGF(meta.BlackName) + "]")
	}
	if meta.WhiteName != "" {
		b.WriteString("PW[" + escapeSGF(meta.WhiteName) + "]")
	}
	if meta.BlackRank != "" {
		b.WriteString("BR[" + escapeSGF(meta.BlackRank) + "]")
	}
	if meta.WhiteRank != "" {
		b.WriteString("WR[" + escapeSGF(meta.WhiteRank) + "]")
	}
	if meta.Event != "" {
		b.WriteString("EV[" + escapeSGF(meta.Event) + "]")
	}
	if !meta.Date.IsZero() {
		b.WriteString("DT[" + meta.Date.Format("2006-01-02") + "]")
	}
	if meta.TimeLimit > 0 {
		fmt.Fprintf(&b, "TM[%d]", meta.TimeLimit)
	}
	if meta.Result != "" {
		b.WriteString("RE[" + escapeSGF(meta.Result) + "]")
	}

	// Handicap stones are setup properties (AB), not moves.
	if s.Config.Handicap > 0 {
		pts := handicapPoints(s.Board.Size(), s.Config.Handicap)
		if len(pts) > 0 {
			b.WriteString("AB")
			for _, p := range pts {
				b.WriteString("[" + coord(p) + "]")
			}
		}
	}

	for _, m := range s.History {
		switch m.Kind {
		case Place:
			if m.Point != nil {
				fmt.Fprintf(&b, ";%s[%s]", m.Player.Short(), coord(*m.Point))
			}
		case Pass:
			// An empty value is SGF's pass.
			fmt.Fprintf(&b, ";%s[]", m.Player.Short())
		case Resign:
			// Resignation is carried in RE[], not as a node; emit a comment so
			// the move list stays readable in other viewers.
			fmt.Fprintf(&b, ";C[%s resigned]", m.Player.Short())
		}
	}
	b.WriteString(")")
	return b.String()
}

// escapeSGF escapes the characters that terminate an SGF property value.
func escapeSGF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "]", "\\]")
}
