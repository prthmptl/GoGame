package profile_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/profile"
	"github.com/prathpatel/gogame-backend/internal/store"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		os.Stderr.WriteString("TEST_DATABASE_URL not set; skipping profile tests\n")
		os.Exit(0)
	}
	st, err := store.Open(context.Background(), dsn, os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		panic(err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		panic(err)
	}
	pool = st.DB
	code := m.Run()
	st.Close()
	os.Exit(code)
}

func newUser(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (display_name, is_guest) VALUES ('T', FALSE) RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func TestPatchLeavesUnsuppliedFieldsAlone(t *testing.T) {
	svc := profile.NewService(pool)
	ctx := context.Background()
	id := newUser(t)

	name := "Prath"
	if _, err := svc.Patch(ctx, id, profile.PatchRequest{DisplayName: &name}); err != nil {
		t.Fatalf("patch name: %v", err)
	}
	country := "IN"
	got, err := svc.Patch(ctx, id, profile.PatchRequest{Country: &country})
	if err != nil {
		t.Fatalf("patch country: %v", err)
	}
	if got.DisplayName != "Prath" {
		t.Errorf("display name = %q, want it preserved", got.DisplayName)
	}
	if got.Country == nil || *got.Country != "IN" {
		t.Errorf("country not applied: %v", got.Country)
	}
	if got.Revision != 2 {
		t.Errorf("revision = %d, want 2 after two patches", got.Revision)
	}
}

func TestSyncAcceptsHigherRevisionAndRejectsStale(t *testing.T) {
	svc := profile.NewService(pool)
	ctx := context.Background()
	id := newUser(t)

	entry := profile.ProgressEntry{
		Kind: "puzzle", ItemID: "p1",
		Payload:  json.RawMessage(`{"status":"solved","mistakes":0}`),
		Revision: 5,
	}
	resp, err := svc.Sync(ctx, id, profile.SyncRequest{Entries: []profile.ProgressEntry{entry}})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if resp.Accepted != 1 {
		t.Fatalf("accepted = %d, want 1", resp.Accepted)
	}

	// A stale device pushing revision 3 must not overwrite revision 5.
	stale := entry
	stale.Revision = 3
	stale.Payload = json.RawMessage(`{"status":"failed"}`)
	resp, err = svc.Sync(ctx, id, profile.SyncRequest{Entries: []profile.ProgressEntry{stale}})
	if err != nil {
		t.Fatalf("sync stale: %v", err)
	}
	if resp.Accepted != 0 || len(resp.Conflicts) != 1 {
		t.Fatalf("accepted=%d conflicts=%d, want 0 and 1", resp.Accepted, len(resp.Conflicts))
	}

	// And the server should still be serving the newer record.
	for _, e := range resp.Entries {
		if e.ItemID == "p1" && e.Revision != 5 {
			t.Errorf("stored revision = %d, want 5", e.Revision)
		}
	}
}

func TestSyncStreakBestNeverRegresses(t *testing.T) {
	svc := profile.NewService(pool)
	ctx := context.Background()
	id := newUser(t)

	day := "2026-08-01"
	if _, err := svc.Sync(ctx, id, profile.SyncRequest{
		Streak: &profile.StreakPayload{Current: 10, Best: 10, LastSolvedOn: &day, Revision: 1},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	// A later revision reporting a lower best (a reinstalled device) must not
	// erase the recorded best.
	resp, err := svc.Sync(ctx, id, profile.SyncRequest{
		Streak: &profile.StreakPayload{Current: 1, Best: 1, LastSolvedOn: &day, Revision: 2},
	})
	if err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if resp.Streak == nil || resp.Streak.Best != 10 {
		t.Fatalf("best streak = %+v, want 10 preserved", resp.Streak)
	}
	if resp.Streak.Current != 1 {
		t.Errorf("current streak = %d, want 1 from the newer revision", resp.Streak.Current)
	}
}

func TestAwardIsIdempotent(t *testing.T) {
	svc := profile.NewService(pool)
	ctx := context.Background()
	id := newUser(t)
	if _, err := pool.Exec(ctx, `
		INSERT INTO achievements (id, title, description) VALUES ('first_win','First win','Win a game')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("seed achievement: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := svc.Award(ctx, id, "first_win", i); err != nil {
			t.Fatalf("award: %v", err)
		}
	}
	list, err := svc.Achievements(ctx, id)
	if err != nil {
		t.Fatalf("achievements: %v", err)
	}
	var found int
	for _, a := range list {
		if a.ID == "first_win" {
			found++
			if !a.Earned {
				t.Error("achievement not marked earned")
			}
			if a.Progress != 2 {
				t.Errorf("progress = %d, want the maximum seen (2)", a.Progress)
			}
		}
	}
	if found != 1 {
		t.Errorf("achievement appeared %d times, want exactly 1", found)
	}
}

func TestLeaderboardRanksByRating(t *testing.T) {
	svc := profile.NewService(pool)
	ctx := context.Background()
	var top uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (display_name, is_guest, rating) VALUES ('Champ', FALSE, 30000)
		RETURNING id`).Scan(&top); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := svc.RefreshLeaderboard(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	rows, err := svc.Leaderboard(ctx, 0, 5)
	if err != nil {
		t.Fatalf("leaderboard: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("leaderboard is empty after refresh")
	}
	if rows[0].UserID != top {
		t.Errorf("rank 1 = %v, want the highest-rated user %v", rows[0].UserID, top)
	}
	if rows[0].Rank != 1 {
		t.Errorf("first row rank = %d, want 1", rows[0].Rank)
	}
}
