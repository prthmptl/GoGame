package goban

import (
	"fmt"
	"math"
)

// Ruleset selects komi, suicide and superko defaults. Mirrors the client's
// Ruleset enum.
type Ruleset string

const (
	Chinese     Ruleset = "chinese"
	Japanese    Ruleset = "japanese"
	Korean      Ruleset = "korean"
	AGA         Ruleset = "aga"
	Ing         Ruleset = "ing"
	NewZealand  Ruleset = "new_zealand"
	TrompTaylor Ruleset = "tromp_taylor"
)

// SuperkoMode selects the repetition rule.
type SuperkoMode string

const (
	// SuperkoNone enforces only the immediate-recapture ko (Japanese, Korean).
	SuperkoNone SuperkoMode = "none"
	// SuperkoPositional forbids repeating any board position.
	SuperkoPositional SuperkoMode = "positional"
	// SuperkoSituational forbids repeating (position, side to move).
	SuperkoSituational SuperkoMode = "situational"
)

// ScoringMethod selects area or territory counting.
type ScoringMethod string

const (
	AreaScoring      ScoringMethod = "area"
	TerritoryScoring ScoringMethod = "territory"
)

// Defaults are the per-ruleset settings, matching RulesetDefaults in the
// client's models.dart.
type Defaults struct {
	Komi          float64
	AllowSuicide  bool
	Superko       SuperkoMode
	ScoringMethod ScoringMethod
}

var rulesetDefaults = map[Ruleset]Defaults{
	Chinese:     {7.5, false, SuperkoPositional, AreaScoring},
	Japanese:    {6.5, false, SuperkoNone, TerritoryScoring},
	Korean:      {6.5, false, SuperkoNone, TerritoryScoring},
	AGA:         {7.5, false, SuperkoSituational, AreaScoring},
	Ing:         {8.0, true, SuperkoSituational, AreaScoring},
	NewZealand:  {7.0, true, SuperkoSituational, AreaScoring},
	TrompTaylor: {7.5, true, SuperkoPositional, AreaScoring},
}

// DefaultsFor returns the settings for a ruleset, falling back to Chinese.
func DefaultsFor(r Ruleset) Defaults {
	if d, ok := rulesetDefaults[r]; ok {
		return d
	}
	return rulesetDefaults[Chinese]
}

// Config is the immutable configuration of one game.
type Config struct {
	BoardSize    int           `json:"boardSize"`
	Ruleset      Ruleset       `json:"ruleset"`
	Komi         float64       `json:"komi"`
	Handicap     int           `json:"handicap"`
	AllowSuicide bool          `json:"allowSuicide"`
	Superko      SuperkoMode   `json:"superkoMode"`
	Scoring      ScoringMethod `json:"scoringMethod"`
}

func (c Config) Validate() error {
	if c.BoardSize != 9 && c.BoardSize != 13 && c.BoardSize != 19 {
		return fmt.Errorf("unsupported board size %d", c.BoardSize)
	}
	if _, ok := rulesetDefaults[c.Ruleset]; !ok {
		return fmt.Errorf("unsupported ruleset %q", c.Ruleset)
	}
	if c.Handicap < 0 || c.Handicap > 9 {
		return fmt.Errorf("handicap must be between 0 and 9")
	}
	if math.IsNaN(c.Komi) || math.IsInf(c.Komi, 0) || math.Abs(c.Komi) > 100 {
		return fmt.Errorf("invalid komi")
	}
	if c.Superko != SuperkoNone && c.Superko != SuperkoPositional && c.Superko != SuperkoSituational {
		return fmt.Errorf("invalid superko mode")
	}
	if c.Scoring != AreaScoring && c.Scoring != TerritoryScoring {
		return fmt.Errorf("invalid scoring method")
	}
	return nil
}

// NewConfig builds a Config with ruleset defaults filled in.
func NewConfig(size int, r Ruleset, handicap int) Config {
	d := DefaultsFor(r)
	return Config{
		BoardSize:    size,
		Ruleset:      r,
		Komi:         d.Komi,
		Handicap:     handicap,
		AllowSuicide: d.AllowSuicide,
		Superko:      d.Superko,
		Scoring:      d.ScoringMethod,
	}
}

// Status is the lifecycle state of a game.
type Status string

const (
	StatusActive   Status = "active"
	StatusScoring  Status = "scoring"
	StatusComplete Status = "completed"
	StatusResigned Status = "resigned"
)

// MoveKind distinguishes placements from passes and resignations.
type MoveKind string

const (
	Place  MoveKind = "place"
	Pass   MoveKind = "pass"
	Resign MoveKind = "resign"
)

// Rejection explains why a move was refused. These strings are the wire
// values the client sees in MOVE_REJECTED.
type Rejection string

const (
	RejGameNotActive Rejection = "game_not_active"
	RejOutOfBounds   Rejection = "out_of_bounds"
	RejOccupied      Rejection = "occupied"
	RejSuicide       Rejection = "suicide"
	RejKo            Rejection = "ko"
	RejSuperko       Rejection = "superko"
	RejNotYourTurn   Rejection = "not_your_turn"
)

// Move is one played move.
type Move struct {
	Number   int      `json:"number"`
	Player   Color    `json:"player"`
	Kind     MoveKind `json:"kind"`
	Point    *Point   `json:"point,omitempty"`
	Captured []Point  `json:"captured,omitempty"`
}

// Intent is a requested move.
type Intent struct {
	Kind  MoveKind
	Point *Point
}

// PlaceAt builds a placement intent.
func PlaceAt(p Point) Intent { return Intent{Kind: Place, Point: &p} }

// PassIntent builds a pass intent.
func PassIntent() Intent { return Intent{Kind: Pass} }

// ResignIntent builds a resignation intent.
func ResignIntent() Intent { return Intent{Kind: Resign} }

// State is an immutable game position plus its history.
type State struct {
	Board             *Board
	Config            Config
	ToMove            Color
	MoveNumber        int
	CapturesByBlack   int
	CapturesByWhite   int
	KoPoint           *Point
	PreviousHashes    map[uint64]struct{}
	Status            Status
	ConsecutivePasses int
	LastMove          *Move
	History           []Move
}

// RejectedError is returned by Apply when a move is illegal.
type RejectedError struct{ Reason Rejection }

func (e *RejectedError) Error() string { return fmt.Sprintf("move rejected: %s", e.Reason) }

// NewGame builds the opening position, placing handicap stones if configured.
func NewGame(cfg Config) *State {
	board := NewBoard(cfg.BoardSize)
	// With handicap stones, black has already played, so white opens.
	toMove := Black
	if cfg.Handicap > 0 {
		toMove = White
		board = board.SetMany(handicapPoints(cfg.BoardSize, cfg.Handicap), Black)
	}
	s := &State{
		Board:          board,
		Config:         cfg,
		ToMove:         toMove,
		PreviousHashes: map[uint64]struct{}{},
		Status:         StatusActive,
		History:        []Move{},
	}
	s.PreviousHashes[positionHash(cfg.Superko, board, toMove)] = struct{}{}
	return s
}

// handicapPoints mirrors GameState._handicapPoints in the client.
func handicapPoints(size, n int) []Point {
	if size != 9 && size != 13 && size != 19 {
		return nil
	}
	edge := 3
	if size == 9 {
		edge = 2
	}
	far := size - 1 - edge
	mid := size / 2
	star := []Point{
		{edge, edge}, {far, far}, {edge, far}, {far, edge}, {mid, mid},
		{mid, edge}, {mid, far}, {edge, mid}, {far, mid},
	}
	if n < 0 {
		n = 0
	}
	if n > 9 {
		n = 9
	}
	return star[:n]
}

// positionHash salts the board hash with the side to move under situational
// superko, matching Rules.positionHash in the client.
func positionHash(mode SuperkoMode, b *Board, toMove Color) uint64 {
	h := b.Zobrist()
	if mode == SuperkoSituational {
		if toMove == Black {
			h ^= 0x1F2E3D4C5B6A7
		} else {
			h ^= 0x7A6B5C4D3E2F1
		}
	}
	return h
}

// Clone returns a deep copy of the state, so callers can explore variations
// without disturbing the live game.
func (s *State) Clone() *State {
	hashes := make(map[uint64]struct{}, len(s.PreviousHashes))
	for k := range s.PreviousHashes {
		hashes[k] = struct{}{}
	}
	history := make([]Move, len(s.History))
	copy(history, s.History)
	var ko *Point
	if s.KoPoint != nil {
		k := *s.KoPoint
		ko = &k
	}
	var last *Move
	if s.LastMove != nil {
		m := *s.LastMove
		last = &m
	}
	return &State{
		Board: s.Board.Clone(), Config: s.Config, ToMove: s.ToMove,
		MoveNumber: s.MoveNumber, CapturesByBlack: s.CapturesByBlack,
		CapturesByWhite: s.CapturesByWhite, KoPoint: ko,
		PreviousHashes: hashes, Status: s.Status,
		ConsecutivePasses: s.ConsecutivePasses, LastMove: last, History: history,
	}
}

// StateHash returns the canonical wire hash of the current position.
func (s *State) StateHash() string { return s.Board.StateHash(s.ToMove) }

// Apply validates an intent against the rules and returns the resulting
// state. The receiver is never mutated.
func (s *State) Apply(in Intent) (*State, *Move, error) {
	if s.Status != StatusActive {
		return nil, nil, &RejectedError{RejGameNotActive}
	}
	switch in.Kind {
	case Pass:
		return s.applyPass()
	case Resign:
		return s.applyResign()
	case Place:
		if in.Point == nil {
			return nil, nil, &RejectedError{RejOutOfBounds}
		}
		return s.applyPlace(*in.Point)
	default:
		return nil, nil, &RejectedError{RejGameNotActive}
	}
}

func (s *State) applyPass() (*State, *Move, error) {
	next := s.Clone()
	passes := s.ConsecutivePasses + 1
	move := Move{Number: s.MoveNumber + 1, Player: s.ToMove, Kind: Pass}

	next.ToMove = s.ToMove.Opposite()
	next.MoveNumber = s.MoveNumber + 1
	next.KoPoint = nil
	next.ConsecutivePasses = passes
	// Two passes in a row move the game into dead-stone marking.
	if passes >= 2 {
		next.Status = StatusScoring
	}
	next.LastMove = &move
	next.History = append(next.History, move)
	return next, &move, nil
}

func (s *State) applyResign() (*State, *Move, error) {
	next := s.Clone()
	move := Move{Number: s.MoveNumber + 1, Player: s.ToMove, Kind: Resign}
	next.Status = StatusResigned
	next.MoveNumber = move.Number
	next.LastMove = &move
	next.History = append(next.History, move)
	return next, &move, nil
}

func (s *State) applyPlace(p Point) (*State, *Move, error) {
	if !s.Board.InBounds(p) {
		return nil, nil, &RejectedError{RejOutOfBounds}
	}
	if s.Board.At(p) != Empty {
		return nil, nil, &RejectedError{RejOccupied}
	}
	if s.KoPoint != nil && *s.KoPoint == p {
		return nil, nil, &RejectedError{RejKo}
	}

	player := s.ToMove
	opponent := player.Opposite()
	placed := s.Board.Set(p, player)

	// Capture adjacent opponent groups left without liberties.
	var captured []Point
	seen := map[Point]bool{}
	for _, n := range placed.Neighbors(p) {
		if placed.At(n) != opponent || seen[n] {
			continue
		}
		g := FindGroup(placed, n)
		for _, st := range g.Stones {
			seen[st] = true
		}
		if len(g.Liberties) == 0 {
			captured = append(captured, g.Stones...)
		}
	}
	afterCapture := placed
	if len(captured) > 0 {
		afterCapture = placed.SetMany(captured, Empty)
	}

	// Suicide: reject, or remove the group under rulesets that permit it.
	ownGroup := FindGroup(afterCapture, p)
	var selfCaptured []Point
	afterMove := afterCapture
	if len(ownGroup.Liberties) == 0 {
		if !s.Config.AllowSuicide {
			return nil, nil, &RejectedError{RejSuicide}
		}
		selfCaptured = ownGroup.Stones
		afterMove = afterCapture.SetMany(selfCaptured, Empty)
	}

	newHash := positionHash(s.Config.Superko, afterMove, opponent)
	if s.Config.Superko != SuperkoNone {
		if _, repeat := s.PreviousHashes[newHash]; repeat {
			return nil, nil, &RejectedError{RejSuperko}
		}
	}

	// Simple ko: a single stone captured a single stone and now has exactly
	// one liberty, so the opponent could otherwise recapture immediately.
	var koPoint *Point
	if len(captured) == 1 && len(selfCaptured) == 0 &&
		len(ownGroup.Stones) == 1 && len(ownGroup.Liberties) == 1 {
		k := captured[0]
		koPoint = &k
	}

	move := Move{
		Number: s.MoveNumber + 1, Player: player, Kind: Place,
		Point: &p, Captured: captured,
	}

	next := s.Clone()
	next.Board = afterMove
	next.ToMove = opponent
	next.MoveNumber = s.MoveNumber + 1
	// Stones a player removes are their captures; stones lost to self-capture
	// count for the opponent.
	if player == Black {
		next.CapturesByBlack += len(captured)
		next.CapturesByWhite += len(selfCaptured)
	} else {
		next.CapturesByWhite += len(captured)
		next.CapturesByBlack += len(selfCaptured)
	}
	next.KoPoint = koPoint
	next.PreviousHashes[newHash] = struct{}{}
	next.ConsecutivePasses = 0
	next.LastMove = &move
	next.History = append(next.History, move)
	return next, &move, nil
}

// LegalPlacements lists every point the side to move may play.
func (s *State) LegalPlacements() []Point {
	if s.Status != StatusActive {
		return nil
	}
	var out []Point
	for r := 0; r < s.Board.Size(); r++ {
		for c := 0; c < s.Board.Size(); c++ {
			p := Point{r, c}
			if s.Board.At(p) != Empty {
				continue
			}
			if _, _, err := s.Apply(PlaceAt(p)); err == nil {
				out = append(out, p)
			}
		}
	}
	return out
}
