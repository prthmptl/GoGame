// Package goban is the server-side Go rules engine.
//
// It is a deliberate parallel implementation of the Flutter client's
// lib/src/domain (board.dart, groups.dart, rules.dart, scoring.dart), as D2
// specifies. The two are kept honest by shared test fixtures in
// testdata/fixtures.json, which both engines execute.
//
// One intentional divergence: the client hashes positions with a Zobrist
// table seeded from Dart's math.Random, whose sequence no other language
// reproduces. Superko is therefore checked with each engine's own internal
// hash, and anything that crosses the wire uses StateHash (SHA-256 over the
// board bytes and side to move), which is identical everywhere.
package goban

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
)

// Color is a stone colour.
type Color uint8

const (
	Empty Color = iota
	Black
	White
)

// Opposite returns the other player.
func (c Color) Opposite() Color {
	switch c {
	case Black:
		return White
	case White:
		return Black
	default:
		return Empty
	}
}

// Short returns the SGF letter for a colour.
func (c Color) Short() string {
	switch c {
	case Black:
		return "B"
	case White:
		return "W"
	default:
		return ""
	}
}

// Point is a board intersection. Row and Col are zero-based from the top-left,
// matching the client's Point.
type Point struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Board is an immutable square board. Every mutator returns a new Board, so a
// game's history can share positions without defensive copying.
type Board struct {
	size  int
	cells []Color
}

// NewBoard returns an empty board of the given size.
func NewBoard(size int) *Board {
	return &Board{size: size, cells: make([]Color, size*size)}
}

// Size returns the board edge length.
func (b *Board) Size() int { return b.size }

// Index maps a coordinate to its slice offset.
func (b *Board) Index(row, col int) int { return row*b.size + col }

// InBounds reports whether p lies on the board.
func (b *Board) InBounds(p Point) bool {
	return p.Row >= 0 && p.Row < b.size && p.Col >= 0 && p.Col < b.size
}

// At returns the colour at p.
func (b *Board) At(p Point) Color { return b.cells[b.Index(p.Row, p.Col)] }

// AtRC returns the colour at (row, col).
func (b *Board) AtRC(row, col int) Color { return b.cells[b.Index(row, col)] }

// Set returns a copy of the board with p set to c.
func (b *Board) Set(p Point, c Color) *Board {
	next := &Board{size: b.size, cells: make([]Color, len(b.cells))}
	copy(next.cells, b.cells)
	next.cells[b.Index(p.Row, p.Col)] = c
	return next
}

// SetMany returns a copy with every listed point set to c.
func (b *Board) SetMany(points []Point, c Color) *Board {
	if len(points) == 0 {
		return b
	}
	next := &Board{size: b.size, cells: make([]Color, len(b.cells))}
	copy(next.cells, b.cells)
	for _, p := range points {
		next.cells[b.Index(p.Row, p.Col)] = c
	}
	return next
}

// Clone returns a deep copy.
func (b *Board) Clone() *Board {
	next := &Board{size: b.size, cells: make([]Color, len(b.cells))}
	copy(next.cells, b.cells)
	return next
}

// Neighbors returns the orthogonally adjacent on-board points.
func (b *Board) Neighbors(p Point) []Point {
	out := make([]Point, 0, 4)
	if p.Row > 0 {
		out = append(out, Point{p.Row - 1, p.Col})
	}
	if p.Row < b.size-1 {
		out = append(out, Point{p.Row + 1, p.Col})
	}
	if p.Col > 0 {
		out = append(out, Point{p.Row, p.Col - 1})
	}
	if p.Col < b.size-1 {
		out = append(out, Point{p.Row, p.Col + 1})
	}
	return out
}

// Equal reports whether two boards hold the same position.
func (b *Board) Equal(other *Board) bool {
	if other == nil || other.size != b.size {
		return false
	}
	for i := range b.cells {
		if b.cells[i] != other.cells[i] {
			return false
		}
	}
	return true
}

// zobristTables are lazily built per board size. Unlike the client's table
// this one is only ever compared against itself, so the seed need not match
// Dart's — see the package comment.
var zobristTables = map[int][]uint64{}

func zobristFor(size int) []uint64 {
	if t, ok := zobristTables[size]; ok {
		return t
	}
	// Fixed seed: the table must be stable across process restarts, or a game
	// reloaded from Postgres would compute different superko hashes than the
	// process that wrote them.
	rng := rand.New(rand.NewPCG(0xCAFEBABE, uint64(size)))
	t := make([]uint64, size*size*2)
	for i := range t {
		t[i] = rng.Uint64()
	}
	zobristTables[size] = t
	return t
}

// Zobrist returns the internal position hash used for superko checks.
func (b *Board) Zobrist() uint64 {
	table := zobristFor(b.size)
	var h uint64
	for i, v := range b.cells {
		if v != Empty {
			h ^= table[i*2+int(v)-1]
		}
	}
	return h
}

// StateHash is the canonical, language-independent position hash sent over
// the wire (D2's "state hash"). Any implementation that hashes the same bytes
// gets the same answer, so the client can verify a reconnect snapshot.
func (b *Board) StateHash(toMove Color) string {
	buf := make([]byte, 0, len(b.cells)+2)
	buf = append(buf, byte(b.size), byte(toMove))
	for _, c := range b.cells {
		buf = append(buf, byte(c))
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
