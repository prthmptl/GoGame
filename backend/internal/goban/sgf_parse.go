package goban

import (
	"fmt"
	"strconv"
	"strings"
)

// SGFGame is a parsed SGF main line.
type SGFGame struct {
	BoardSize int
	Komi      float64
	Handicap  int
	Ruleset   string
	BlackName string
	WhiteName string
	BlackRank string
	WhiteRank string
	Event     string
	Round     string
	Place     string
	Date      string
	Result    string
	// Setup stones placed by AB/AW before play.
	SetupBlack []Point
	SetupWhite []Point
	Moves      []Intent
	MoveColors []Color
}

// ParseSGF reads the main line of an SGF file.
//
// Variations are skipped: everything inside a nested branch is ignored, which
// is what "main line" means and what both the archive and the opening indexer
// need. A full variation tree lives on the client for review.
func ParseSGF(input string) (*SGFGame, error) {
	g := &SGFGame{BoardSize: 19, Komi: 6.5, Ruleset: string(Japanese)}

	i := 0
	depth := 0
	n := len(input)
	for i < n {
		switch input[i] {
		case '(':
			depth++
			i++
			continue
		case ')':
			depth--
			i++
			if depth <= 0 {
				return finish(g)
			}
			continue
		case ';':
			i++
			continue
		}
		if !isUpper(input[i]) {
			i++
			continue
		}

		// Read the property identifier.
		start := i
		for i < n && isUpper(input[i]) {
			i++
		}
		ident := input[start:i]

		var values []string
		for i < n {
			for i < n && (input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r') {
				i++
			}
			if i >= n || input[i] != '[' {
				break
			}
			i++
			var sb strings.Builder
			for i < n && input[i] != ']' {
				if input[i] == '\\' && i+1 < n {
					i++
					sb.WriteByte(input[i])
					i++
					continue
				}
				sb.WriteByte(input[i])
				i++
			}
			if i < n {
				i++ // consume ']'
			}
			values = append(values, sb.String())
		}
		if len(values) == 0 {
			continue
		}

		// Only the first branch contributes to the main line.
		if depth > 1 && (ident == "B" || ident == "W") {
			continue
		}
		applyProperty(g, ident, values)
	}
	return finish(g)
}

func finish(g *SGFGame) (*SGFGame, error) {
	if g.BoardSize < 2 || g.BoardSize > 25 {
		return nil, fmt.Errorf("unsupported board size %d", g.BoardSize)
	}
	return g, nil
}

func applyProperty(g *SGFGame, ident string, values []string) {
	v := values[0]
	switch ident {
	case "SZ":
		// "19" or "19:19"; rectangular boards are not supported.
		if idx := strings.IndexByte(v, ':'); idx >= 0 {
			v = v[:idx]
		}
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			g.BoardSize = n
		}
	case "KM":
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			g.Komi = f
		}
	case "HA":
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			g.Handicap = n
		}
	case "RU":
		g.Ruleset = v
	case "PB":
		g.BlackName = v
	case "PW":
		g.WhiteName = v
	case "BR":
		g.BlackRank = v
	case "WR":
		g.WhiteRank = v
	case "EV":
		g.Event = v
	case "RO":
		g.Round = v
	case "PC":
		g.Place = v
	case "DT":
		g.Date = v
	case "RE":
		g.Result = v
	case "AB":
		for _, val := range values {
			if p, ok := parseCoord(val, g.BoardSize); ok {
				g.SetupBlack = append(g.SetupBlack, p)
			}
		}
	case "AW":
		for _, val := range values {
			if p, ok := parseCoord(val, g.BoardSize); ok {
				g.SetupWhite = append(g.SetupWhite, p)
			}
		}
	case "B", "W":
		color := Black
		if ident == "W" {
			color = White
		}
		// An empty value, or "tt" on boards up to 19x19, is a pass.
		if v == "" || (v == "tt" && g.BoardSize <= 19) {
			g.Moves = append(g.Moves, PassIntent())
			g.MoveColors = append(g.MoveColors, color)
			return
		}
		if p, ok := parseCoord(v, g.BoardSize); ok {
			g.Moves = append(g.Moves, PlaceAt(p))
			g.MoveColors = append(g.MoveColors, color)
		}
	}
}

// parseCoord decodes SGF's column-major letter pair.
func parseCoord(v string, size int) (Point, bool) {
	if len(v) < 2 {
		return Point{}, false
	}
	col := int(v[0] - 'a')
	row := int(v[1] - 'a')
	if col < 0 || col >= size || row < 0 || row >= size {
		return Point{}, false
	}
	return Point{Row: row, Col: col}, true
}

// Config builds the engine configuration this SGF describes.
func (g *SGFGame) Config() Config {
	rs := Ruleset(strings.ToLower(strings.TrimSpace(g.Ruleset)))
	if _, ok := rulesetDefaults[rs]; !ok {
		// Unknown or absent RU: Japanese is the near-universal default in
		// archived professional records.
		rs = Japanese
	}
	cfg := NewConfig(g.BoardSize, rs, g.Handicap)
	cfg.Komi = g.Komi
	return cfg
}

// Winner extracts "black", "white" or "draw" from the RE property.
func (g *SGFGame) Winner() string {
	r := strings.ToUpper(strings.TrimSpace(g.Result))
	switch {
	case strings.HasPrefix(r, "B+"):
		return "black"
	case strings.HasPrefix(r, "W+"):
		return "white"
	case r == "0" || strings.HasPrefix(r, "DRAW") || r == "JIGO":
		return "draw"
	default:
		return ""
	}
}

func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }
