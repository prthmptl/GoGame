package goban

// Group is a connected set of same-coloured stones and its liberties.
type Group struct {
	Stones    []Point
	Liberties []Point
}

// FindGroup flood-fills the group containing start. Calling it on an empty
// point returns an empty group rather than panicking, since callers reach it
// from scoring paths where emptiness is expected.
func FindGroup(b *Board, start Point) Group {
	color := b.At(start)
	if color == Empty {
		return Group{}
	}
	stones := map[Point]bool{}
	libs := map[Point]bool{}
	stack := []Point{start}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if stones[p] {
			continue
		}
		stones[p] = true
		for _, n := range b.Neighbors(p) {
			switch b.At(n) {
			case Empty:
				libs[n] = true
			case color:
				if !stones[n] {
					stack = append(stack, n)
				}
			}
		}
	}
	return Group{Stones: sortedPoints(stones), Liberties: sortedPoints(libs)}
}

// Liberties returns the liberty count of the group at p.
func Liberties(b *Board, p Point) int {
	if b.At(p) == Empty {
		return 0
	}
	return len(FindGroup(b, p).Liberties)
}

// sortedPoints returns map keys in a deterministic row-major order, so move
// records and state hashes never depend on Go's map iteration order.
func sortedPoints(set map[Point]bool) []Point {
	out := make([]Point, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && less(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func less(a, b Point) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}
