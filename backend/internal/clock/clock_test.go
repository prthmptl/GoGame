package clock

import (
	"testing"
	"time"
)

func TestAbsoluteFlagsWhenMainTimeRunsOut(t *testing.T) {
	c := New(Absolute(10), "black") // 10 seconds
	if got := c.Tick(9_000); got != "" {
		t.Fatalf("flagged early: %s", got)
	}
	if c.Black.MainMillis != 1_000 {
		t.Errorf("main = %d, want 1000", c.Black.MainMillis)
	}
	if got := c.Tick(1_500); got != "black" {
		t.Fatalf("flagged = %q, want black", got)
	}
	if !c.Black.Flagged {
		t.Error("black should be flagged")
	}
}

func TestFischerAddsIncrementOnMove(t *testing.T) {
	c := New(Fischer(60, 5), "black")
	c.Tick(10_000)
	c.OnMovePlayed("black")
	if want := 55_000; c.Black.MainMillis != want {
		t.Errorf("main = %d, want %d (60s - 10s + 5s)", c.Black.MainMillis, want)
	}
	if c.Active != "white" {
		t.Errorf("active = %s, want white after black moved", c.Active)
	}
}

func TestFischerDoesNotChargeTheIdlePlayer(t *testing.T) {
	c := New(Fischer(60, 5), "black")
	c.Tick(20_000)
	if c.White.MainMillis != 60_000 {
		t.Errorf("white main = %d, want untouched 60000", c.White.MainMillis)
	}
}

func TestByoYomiEntersOvertimeAndResetsOnMove(t *testing.T) {
	c := New(ByoYomi(10, 3, 30), "black") // 10s main, 3 x 30s
	// Overrun main time by 5s; that comes out of the first period.
	c.Tick(15_000)
	if !c.Black.InOvertime {
		t.Fatal("expected byo-yomi to start")
	}
	if c.Black.MainMillis != 0 {
		t.Errorf("main = %d, want 0", c.Black.MainMillis)
	}
	if want := 25_000; c.Black.PeriodMillis != want {
		t.Errorf("period = %d, want %d (30s - 5s overflow)", c.Black.PeriodMillis, want)
	}
	if c.Black.PeriodsLeft != 3 {
		t.Errorf("periods left = %d, want 3 (none consumed yet)", c.Black.PeriodsLeft)
	}

	// Playing within the period refills it — that is the point of byo-yomi.
	c.OnMovePlayed("black")
	if c.Black.PeriodMillis != 30_000 {
		t.Errorf("period after move = %d, want a full 30000", c.Black.PeriodMillis)
	}
}

func TestByoYomiConsumesPeriodsThenFlags(t *testing.T) {
	c := New(ByoYomi(0, 2, 10), "black") // straight into 2 x 10s
	// 25s with no moves: burns period 1 (10s), period 2 (10s), then flags.
	if got := c.Tick(25_000); got != "black" {
		t.Fatalf("flagged = %q, want black after exhausting every period", got)
	}
	if c.Black.PeriodsLeft != 0 {
		t.Errorf("periods left = %d, want 0", c.Black.PeriodsLeft)
	}
}

func TestByoYomiRollsOverflowIntoNextPeriod(t *testing.T) {
	c := New(ByoYomi(0, 3, 10), "black")
	// 12s consumes the first period and 2s of the second.
	if got := c.Tick(12_000); got != "" {
		t.Fatalf("flagged too early: %s", got)
	}
	if c.Black.PeriodsLeft != 2 {
		t.Errorf("periods left = %d, want 2", c.Black.PeriodsLeft)
	}
	if want := 8_000; c.Black.PeriodMillis != want {
		t.Errorf("period = %d, want %d", c.Black.PeriodMillis, want)
	}
}

func TestCanadianRefillsOnlyAfterTheStoneQuota(t *testing.T) {
	c := New(Canadian(0, 3, 30), "black") // 3 stones per 30s
	c.Tick(5_000)
	if !c.Black.InOvertime {
		t.Fatal("expected Canadian overtime")
	}
	// First two moves consume stones without refilling the period.
	c.OnMovePlayed("black")
	if c.Black.StonesLeftInPeriod != 2 {
		t.Errorf("stones left = %d, want 2", c.Black.StonesLeftInPeriod)
	}
	if c.Black.PeriodMillis == 30_000 {
		t.Error("period refilled before the stone quota was met")
	}
	c.SetActive("black")
	c.OnMovePlayed("black")
	c.SetActive("black")
	c.OnMovePlayed("black")
	// The third stone completes the quota and refills.
	if c.Black.StonesLeftInPeriod != 3 {
		t.Errorf("stones left = %d, want a reset to 3", c.Black.StonesLeftInPeriod)
	}
	if c.Black.PeriodMillis != 30_000 {
		t.Errorf("period = %d, want a full 30000 after the quota", c.Black.PeriodMillis)
	}
}

func TestNoControlNeverFlags(t *testing.T) {
	c := New(NoControl(), "black")
	if got := c.Tick(10 * 60 * 60 * 1000); got != "" {
		t.Errorf("untimed game flagged %q", got)
	}
}

func TestRestoreChargesTimeSpentOffline(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := New(Absolute(60), "black")
	state := c.Export(start)

	// The process was down for 20 seconds; black was on move throughout.
	restored := Restore(state, start.Add(20*time.Second))
	if want := 40_000; restored.Black.MainMillis != want {
		t.Errorf("main after restore = %d, want %d — a restart must not gift time",
			restored.Black.MainMillis, want)
	}
	if restored.White.MainMillis != 60_000 {
		t.Errorf("white was charged for the outage: %d", restored.White.MainMillis)
	}
}

func TestRestoreCanFlagAPlayerWhoTimedOutWhileOffline(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := New(Absolute(10), "black").Export(start)
	restored := Restore(state, start.Add(30*time.Second))
	if !restored.Black.Flagged {
		t.Error("black should be flagged after being on move for 30s of a 10s budget")
	}
}

func TestRemainingMillisAccountsForPeriods(t *testing.T) {
	c := New(ByoYomi(60, 3, 30), "black")
	// 60s main + 3 x 30s = 150s.
	if want := 150_000; c.RemainingMillis("black") != want {
		t.Errorf("remaining = %d, want %d", c.RemainingMillis("black"), want)
	}
	c.Tick(70_000) // into overtime
	// 20s left in the current period + 2 further periods = 80s.
	if want := 80_000; c.RemainingMillis("black") != want {
		t.Errorf("remaining in overtime = %d, want %d", c.RemainingMillis("black"), want)
	}
}

func TestTimeClassBuckets(t *testing.T) {
	cases := []struct {
		c    Control
		want string
	}{
		{Absolute(60), "blitz"},
		{Absolute(600), "rapid"},
		{Absolute(3600), "classical"},
		{Fischer(120, 2), "rapid"}, // 120 + 40*2 = 200s
		{ByoYomi(1800, 5, 30), "classical"},
		{NoControl(), "unlimited"},
	}
	for _, c := range cases {
		if got := c.c.TimeClass(); got != c.want {
			t.Errorf("%s TimeClass() = %s, want %s", c.c.Describe(), got, c.want)
		}
	}
}
