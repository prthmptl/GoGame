// Package clock implements D3: server-authoritative game clocks.
//
// This is a port of the Flutter client's
// lib/src/domain/clock/clock_controller.dart, with two changes required for
// server use: snapshots serialise to JSON so a game survives a process
// restart, and every transition is driven by an explicit timestamp rather
// than a local wall clock, so replaying a game yields identical results.
package clock

import (
	"encoding/json"
	"fmt"
	"time"
)

// Kind is the time-control family.
type Kind string

const (
	KindNone     Kind = "none"
	KindAbsolute Kind = "absolute"
	KindFischer  Kind = "fischer"
	KindByoYomi  Kind = "byo_yomi"
	KindCanadian Kind = "canadian"
)

// Control is an immutable time-control setting, mirroring the client's
// TimeControl.
type Control struct {
	Kind             Kind `json:"kind"`
	MainSeconds      int  `json:"mainSeconds"`
	IncrementSeconds int  `json:"incrementSeconds"`
	PeriodSeconds    int  `json:"periodSeconds"`
	Periods          int  `json:"periods"`
	StonesPerPeriod  int  `json:"stonesPerPeriod"`
}

// Validate bounds client-supplied controls before any clock loop or arithmetic.
func (c Control) Validate() error {
	if c.MainSeconds < 0 || c.MainSeconds > 86400 || c.IncrementSeconds < 0 || c.IncrementSeconds > 3600 ||
		c.PeriodSeconds < 0 || c.PeriodSeconds > 3600 || c.Periods < 0 || c.Periods > 100 ||
		c.StonesPerPeriod < 0 || c.StonesPerPeriod > 361 {
		return fmt.Errorf("time control values out of range")
	}
	switch c.Kind {
	case KindNone:
		return nil
	case KindAbsolute, KindFischer:
		if c.MainSeconds > 0 {
			return nil
		}
	case KindByoYomi:
		if c.PeriodSeconds > 0 && c.Periods > 0 {
			return nil
		}
	case KindCanadian:
		if c.PeriodSeconds > 0 && c.StonesPerPeriod > 0 {
			return nil
		}
	}
	return fmt.Errorf("invalid time control")
}

// NoControl is an untimed game.
func NoControl() Control { return Control{Kind: KindNone} }

// Absolute is a single main-time budget.
func Absolute(mainSeconds int) Control {
	return Control{Kind: KindAbsolute, MainSeconds: mainSeconds}
}

// Fischer adds increment after every move.
func Fischer(mainSeconds, incrementSeconds int) Control {
	return Control{Kind: KindFischer, MainSeconds: mainSeconds, IncrementSeconds: incrementSeconds}
}

// ByoYomi grants a number of fixed-length periods after main time.
func ByoYomi(mainSeconds, periods, periodSeconds int) Control {
	return Control{Kind: KindByoYomi, MainSeconds: mainSeconds, Periods: periods, PeriodSeconds: periodSeconds}
}

// Canadian grants a stone allowance per period after main time.
func Canadian(mainSeconds, stonesPerPeriod, periodSeconds int) Control {
	return Control{Kind: KindCanadian, MainSeconds: mainSeconds,
		StonesPerPeriod: stonesPerPeriod, PeriodSeconds: periodSeconds}
}

// TimeClass buckets a control for rating and matchmaking (E1, D4). The
// thresholds follow the usual online-Go convention on estimated game length.
func (c Control) TimeClass() string {
	switch c.Kind {
	case KindNone:
		return "unlimited"
	case KindCanadian, KindByoYomi:
		// Overtime games are judged on main time plus one full period set.
		total := c.MainSeconds + c.Periods*c.PeriodSeconds
		if c.Kind == KindCanadian {
			total = c.MainSeconds + c.PeriodSeconds
		}
		return classify(total)
	default:
		return classify(c.MainSeconds + 40*c.IncrementSeconds)
	}
}

func classify(seconds int) string {
	switch {
	case seconds < 180:
		return "blitz"
	case seconds < 1500:
		return "rapid"
	default:
		return "classical"
	}
}

// Describe renders a short human-readable label.
func (c Control) Describe() string {
	mins := c.MainSeconds / 60
	switch c.Kind {
	case KindNone:
		return "No clock"
	case KindAbsolute:
		return fmt.Sprintf("%d min", mins)
	case KindFischer:
		return fmt.Sprintf("%d min + %ds", mins, c.IncrementSeconds)
	case KindByoYomi:
		return fmt.Sprintf("%d min · %d × %ds byo-yomi", mins, c.Periods, c.PeriodSeconds)
	case KindCanadian:
		return fmt.Sprintf("%d min · %d/%ds Canadian", mins, c.StonesPerPeriod, c.PeriodSeconds)
	}
	return string(c.Kind)
}

// Snapshot is one player's clock at an instant. Mirrors the client's
// ClockSnapshot, plus JSON tags so it can be persisted and sent in
// CLOCK_UPDATE.
type Snapshot struct {
	MainMillis         int  `json:"mainMillis"`
	PeriodMillis       int  `json:"periodMillis"`
	PeriodsLeft        int  `json:"periodsLeft"`
	StonesLeftInPeriod int  `json:"stonesLeftInPeriod"`
	InOvertime         bool `json:"inOvertime"`
	Flagged            bool `json:"flagged"`
}

// Controller holds both players' clocks and applies elapsed time and moves.
type Controller struct {
	Control Control
	Black   Snapshot
	White   Snapshot
	Active  string // "black" | "white"
}

// New builds a controller with both clocks at their starting values.
func New(c Control, active string) *Controller {
	return &Controller{Control: c, Black: seed(c), White: seed(c), Active: active}
}

func seed(c Control) Snapshot {
	if c.Kind == KindNone {
		return Snapshot{}
	}
	return Snapshot{
		MainMillis:         c.MainSeconds * 1000,
		PeriodMillis:       c.PeriodSeconds * 1000,
		PeriodsLeft:        c.Periods,
		StonesLeftInPeriod: c.StonesPerPeriod,
	}
}

// Tick charges elapsed milliseconds to the active player. It returns the
// colour that just ran out of time, or "" if neither did.
func (c *Controller) Tick(elapsedMillis int) string {
	if c.Control.Kind == KindNone || elapsedMillis <= 0 {
		return ""
	}
	updated := c.applyTick(c.get(c.Active), elapsedMillis)
	c.set(c.Active, updated)
	if updated.Flagged {
		return c.Active
	}
	return ""
}

// OnMovePlayed applies the mover's increment or period reset and hands the
// clock to the opponent.
func (c *Controller) OnMovePlayed(mover string) {
	c.set(mover, c.applyMoveBonus(c.get(mover)))
	c.Active = other(mover)
}

// SetActive forces which clock is running, used when restoring a game.
func (c *Controller) SetActive(color string) { c.Active = color }

// RemainingMillis reports how long the given player has before flagging. The
// timeout watcher schedules its next wake-up from this.
func (c *Controller) RemainingMillis(color string) int {
	if c.Control.Kind == KindNone {
		return 1 << 30
	}
	s := c.get(color)
	if s.Flagged {
		return 0
	}
	if !s.InOvertime {
		remaining := s.MainMillis
		switch c.Control.Kind {
		case KindByoYomi:
			remaining += s.PeriodsLeft * c.Control.PeriodSeconds * 1000
		case KindCanadian:
			remaining += s.PeriodMillis
		}
		return remaining
	}
	if c.Control.Kind == KindByoYomi {
		// The current period plus every period still in hand.
		return s.PeriodMillis + (s.PeriodsLeft-1)*c.Control.PeriodSeconds*1000
	}
	return s.PeriodMillis
}

func (c *Controller) get(color string) Snapshot {
	if color == "black" {
		return c.Black
	}
	return c.White
}

func (c *Controller) set(color string, s Snapshot) {
	if color == "black" {
		c.Black = s
	} else {
		c.White = s
	}
}

func other(color string) string {
	if color == "black" {
		return "white"
	}
	return "black"
}

func (c *Controller) applyTick(s Snapshot, elapsed int) Snapshot {
	if s.Flagged {
		return s
	}
	if !s.InOvertime {
		main := s.MainMillis - elapsed
		if main > 0 {
			s.MainMillis = main
			return s
		}
		overflow := -main
		s.MainMillis = 0
		switch c.Control.Kind {
		case KindAbsolute, KindFischer, KindNone:
			s.Flagged = true
			return s
		case KindByoYomi, KindCanadian:
			// Time spent past main time is charged to the first period.
			s.InOvertime = true
			return c.consumeOvertime(s, overflow)
		}
	}
	return c.consumeOvertime(s, elapsed)
}

func (c *Controller) consumeOvertime(s Snapshot, elapsed int) Snapshot {
	remaining := s.PeriodMillis - elapsed
	for remaining <= 0 {
		switch c.Control.Kind {
		case KindByoYomi:
			s.PeriodsLeft--
			if s.PeriodsLeft <= 0 {
				s.PeriodMillis = 0
				s.PeriodsLeft = 0
				s.Flagged = true
				return s
			}
			// Roll the overflow into the next fresh period.
			remaining += c.Control.PeriodSeconds * 1000
		case KindCanadian:
			// Failing to play the required stones inside the period flags.
			s.PeriodMillis = 0
			s.Flagged = true
			return s
		default:
			s.PeriodMillis = 0
			s.Flagged = true
			return s
		}
	}
	s.PeriodMillis = remaining
	return s
}

func (c *Controller) applyMoveBonus(s Snapshot) Snapshot {
	if c.Control.Kind == KindNone || s.Flagged {
		return s
	}
	switch c.Control.Kind {
	case KindAbsolute:
		return s
	case KindFischer:
		s.MainMillis += c.Control.IncrementSeconds * 1000
		return s
	case KindByoYomi:
		if !s.InOvertime {
			return s
		}
		// Completing a move in byo-yomi refills the period.
		s.PeriodMillis = c.Control.PeriodSeconds * 1000
		return s
	case KindCanadian:
		if !s.InOvertime {
			return s
		}
		s.StonesLeftInPeriod--
		if s.StonesLeftInPeriod <= 0 {
			s.StonesLeftInPeriod = c.Control.StonesPerPeriod
			s.PeriodMillis = c.Control.PeriodSeconds * 1000
		}
		return s
	}
	return s
}

// State is the serialisable form of a controller, so a game can be restored
// after a restart or handed to another instance.
type State struct {
	Control Control  `json:"control"`
	Black   Snapshot `json:"black"`
	White   Snapshot `json:"white"`
	Active  string   `json:"active"`
	// UpdatedAt is when the snapshots were last charged; the elapsed time
	// since then must be applied on restore.
	UpdatedAt time.Time `json:"updatedAt"`
}

// Export captures the controller for persistence.
func (c *Controller) Export(now time.Time) State {
	return State{Control: c.Control, Black: c.Black, White: c.White,
		Active: c.Active, UpdatedAt: now}
}

// Restore rebuilds a controller and charges the time that passed while the
// game was not in memory, so a server restart cannot hand a player free time.
func Restore(s State, now time.Time) *Controller {
	c := &Controller{Control: s.Control, Black: s.Black, White: s.White, Active: s.Active}
	if !s.UpdatedAt.IsZero() {
		if elapsed := int(now.Sub(s.UpdatedAt).Milliseconds()); elapsed > 0 {
			c.Tick(elapsed)
		}
	}
	return c
}

// MarshalJSON lets a State be stored directly in a JSONB column.
func (c *Controller) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Export(time.Now().UTC()))
}
