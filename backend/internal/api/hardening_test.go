package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/blob"
	"github.com/prathpatel/gogame-backend/internal/chat"
	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/goban"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/rating"
	"github.com/prathpatel/gogame-backend/internal/store"
	"github.com/prathpatel/gogame-backend/internal/ws"
)

func TestDecodeJSONRejectsAmbiguousBodies(t *testing.T) {
	for _, body := range []string{`null`, `{} {}`, `{"unexpected":1}`, `{}` + strings.Repeat(" ", 1<<20)} {
		var dst struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(httptest.NewRequest("POST", "/", strings.NewReader(body)), &dst); err == nil {
			t.Fatalf("accepted invalid body starting %.30q", body)
		}
	}
	var dst struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"Go"}`)), &dst); err != nil {
		t.Fatal(err)
	}
}

func TestClientIPRequiresTrustedProxyAndIgnoresSpoofedPrefix(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "192.0.2.2:1234"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 198.51.100.4")
	r.Header.Set("Fly-Client-IP", "2.2.2.2")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	if got := clientIP(r); got != "192.0.2.2" {
		t.Fatal(got)
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.0/24")
	if got := clientIP(r); got != "198.51.100.4" {
		t.Fatal(got)
	}
}

func integrationStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	st, err := store.Open(context.Background(), dsn, os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}
func user(t *testing.T, st *store.Store) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := st.DB.QueryRow(context.Background(), `INSERT INTO users(display_name,is_guest) VALUES ('Test',false) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func newSession(t *testing.T, st *store.Store, h *game.Hub) (*game.Session, uuid.UUID, uuid.UUID) {
	t.Helper()
	b, w := user(t, st), user(t, st)
	s, err := h.Create(context.Background(), game.Config{Black: game.Player{UserID: &b}, White: game.Player{UserID: &w}, Rules: goban.NewConfig(9, goban.Chinese, 0), TimeControl: clock.NoControl(), Mode: "ranked"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.Release(context.Background(), s.ID)
		st.DB.Exec(context.Background(), `DELETE FROM games WHERE id=$1`, s.ID)
	})
	return s, b, w
}

func TestAuthRateLimitAndUnconfiguredWebhook(t *testing.T) {
	st := integrationStore(t)
	server := New(Deps{Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	handler := server.limitAuth(2, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	path := "/auth/" + uuid.NewString()
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", path, nil)
		handler(w, r)
		want := http.StatusNoContent
		if i == 2 {
			want = http.StatusTooManyRequests
		}
		if w.Code != want {
			t.Fatalf("status %d want %d", w.Code, want)
		}
	}
	w := httptest.NewRecorder()
	server.handleStoreWebhook(w, httptest.NewRequest("POST", "/webhooks/store/ios", strings.NewReader(`{"notificationUID":"spoof"}`)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal(w.Code)
	}
}

func TestWebsocketUsesModeratedChatAndRejectsIdentityChanges(t *testing.T) {
	st := integrationStore(t)
	h := game.NewHub(st.DB, st.Redis, uuid.NewString(), game.Hooks{})
	t.Cleanup(h.Close)
	sess, b, w := newSession(t, st, h)
	secret := []byte("test-secret-at-least-32-bytes-long!!")
	authSvc := auth.NewService(auth.Options{DB: st.DB, JWTSecret: secret, Issuer: "test", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	sign := func(id uuid.UUID) string {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: id.String(), Issuer: "test", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}).SignedString(secret)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	chats := chat.NewService(st.DB, st.Redis)
	socket := ws.NewServer(authSvc, h, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	socket.Chat = chats
	socket.Archive = archive.NewService(st.DB, blob.NewMemory())
	socket.Anticheat = anticheat.NewService(st.DB)
	httpServer := httptest.NewServer(http.HandlerFunc(socket.Handle))
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpServer.URL, "http")+"?token="+sign(b), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	read := func() ws.Envelope {
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var env ws.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		return env
	}
	send := func(kind string, payload any) {
		raw, _ := json.Marshal(payload)
		data, _ := json.Marshal(ws.Envelope{Type: kind, Payload: raw})
		if err := c.Write(ctx, websocket.MessageText, data); err != nil {
			t.Fatal(err)
		}
	}
	if got := read().Type; got != "AUTHENTICATED" {
		t.Fatal(got)
	}
	send("JOIN_GAME", map[string]any{"gameId": sess.ID.String()})
	if got := read().Type; got != "GAME_SNAPSHOT" {
		t.Fatal(got)
	}
	send("CHAT_MESSAGE", map[string]any{"text": "shit"})
	env := read()
	if env.Type != "CHAT_RECEIVED" || !strings.Contains(string(env.Payload), "****") {
		t.Fatalf("unfiltered chat: %s %s", env.Type, env.Payload)
	}
	history, err := chats.History(ctx, sess.ID, 10)
	if err != nil || len(history) != 1 || history[0].Body != "****" {
		t.Fatalf("history %v %v", history, err)
	}
	send("AUTHENTICATE", map[string]any{"token": sign(w)})
	if env := read(); env.Type != "ERROR" {
		t.Fatalf("identity changed: %s", env.Type)
	}
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("connection survived identity change")
	}
}

type failingBlob struct{ *blob.Memory }

func (failingBlob) Put(context.Context, string, []byte, string) error {
	return errors.New("storage down")
}

func TestCompletionRetriesWithoutDoubleRatingOrNotifications(t *testing.T) {
	st := integrationStore(t)
	h := game.NewHub(st.DB, st.Redis, uuid.NewString(), game.Hooks{})
	t.Cleanup(h.Close)
	sess, b, w := newSession(t, st, h)
	if err := sess.Resign(b, 1); err != nil {
		t.Fatal(err)
	}
	ratings, cheats := rating.NewService(st.DB), anticheat.NewService(st.DB)
	notifier := notify.NewService(st.DB, nil)
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	bad := archive.NewService(st.DB, failingBlob{blob.NewMemory()})
	if err := processCompletion(ctx, st.DB, ratings, cheats, bad, notifier, lg, &sess.ID); err == nil {
		t.Fatal("expected storage failure")
	}
	good := archive.NewService(st.DB, blob.NewMemory())
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = processCompletion(ctx, st.DB, ratings, cheats, good, notifier, lg, &sess.ID)
		}()
	}
	wg.Wait()
	for _, id := range []uuid.UUID{b, w} {
		row, err := ratings.Get(ctx, id, 9, "unlimited")
		if err != nil || row.Games != 1 {
			t.Fatalf("rating applied repeatedly: %+v %v", row, err)
		}
	}
	var pending, notifications int
	_ = st.DB.QueryRow(ctx, `SELECT count(*) FROM game_completion_outbox WHERE game_id=$1`, sess.ID).Scan(&pending)
	_ = st.DB.QueryRow(ctx, `SELECT count(*) FROM notification_outbox WHERE data->>'gameId'=$1`, sess.ID.String()).Scan(&notifications)
	if pending != 0 || notifications != 2 {
		t.Fatalf("pending=%d notifications=%d", pending, notifications)
	}
	delivered, failed, err := notifier.DeliverBatch(ctx, 100)
	if err != nil || delivered != 0 || failed != 0 {
		t.Fatal("unconfigured sender consumed notifications")
	}
}
