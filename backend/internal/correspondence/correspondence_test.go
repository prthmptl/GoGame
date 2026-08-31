package correspondence

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/goban"
)

func TestResolvePlanPlaysThePreparedReply(t *testing.T) {
	p := &Plan{
		GameID: uuid.New(), UserID: uuid.New(), FromMove: 10,
		Sequence: []Branch{
			{If: goban.Point{Row: 3, Col: 3}, Then: goban.Point{Row: 4, Col: 4}},
			{If: goban.Point{Row: 5, Col: 5}, Then: goban.Point{Row: 6, Col: 6}},
		},
	}
	reply, rest, err := ResolvePlan(p, 10, goban.Point{Row: 3, Col: 3})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reply == nil || *reply != (goban.Point{Row: 4, Col: 4}) {
		t.Fatalf("reply = %v, want (4,4)", reply)
	}
	if len(rest) != 1 {
		t.Errorf("remaining branches = %d, want 1", len(rest))
	}
}

func TestResolvePlanIsVoidedByAnUnexpectedMove(t *testing.T) {
	p := &Plan{
		FromMove: 10,
		Sequence: []Branch{{If: goban.Point{Row: 3, Col: 3}, Then: goban.Point{Row: 4, Col: 4}}},
	}
	// The opponent played somewhere else entirely.
	reply, _, err := ResolvePlan(p, 10, goban.Point{Row: 15, Col: 15})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if reply != nil {
		t.Errorf("reply = %v, want nil when the condition did not hold", reply)
	}
}

func TestResolvePlanRefusesAStalePlan(t *testing.T) {
	p := &Plan{
		FromMove: 10,
		Sequence: []Branch{{If: goban.Point{Row: 3, Col: 3}, Then: goban.Point{Row: 4, Col: 4}}},
	}
	// The game has moved on; firing this plan now would play into a position
	// its author never saw.
	_, _, err := ResolvePlan(p, 14, goban.Point{Row: 3, Col: 3})
	if !errors.Is(err, ErrStalePlan) {
		t.Errorf("err = %v, want ErrStalePlan", err)
	}
}

func TestResolvePlanHandlesNoPlan(t *testing.T) {
	reply, rest, err := ResolvePlan(nil, 1, goban.Point{Row: 0, Col: 0})
	if err != nil || reply != nil || rest != nil {
		t.Errorf("empty plan returned (%v, %v, %v), want all nil", reply, rest, err)
	}
	empty := &Plan{FromMove: 1}
	if reply, _, err := ResolvePlan(empty, 1, goban.Point{Row: 0, Col: 0}); err != nil || reply != nil {
		t.Errorf("plan with no branches returned (%v, %v)", reply, err)
	}
}

func TestStatePausedReflectsVacation(t *testing.T) {
	var s State
	if s.Paused() {
		t.Error("a state with no pause timestamp reported paused")
	}
	now := s.LastMoveAt
	s.PausedAt = &now
	if !s.Paused() {
		t.Error("a paused state reported running")
	}
}

func TestVacationActiveFlag(t *testing.T) {
	v := Vacation{DaysRemaining: 30}
	if v.Active() {
		t.Error("a player with no active_since reported as away")
	}
}
