// Package game implements D2: the game-session service.
//
// Each live game is owned by exactly one goroutine (the "actor"). Every
// mutation arrives on that goroutine's command channel, so the rules engine,
// the clock and the persistence writes all happen without locks and without
// two moves ever interleaving.
package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// snapshotMoves is how many recent moves a reconnect snapshot carries, per D2.
const snapshotMoves = 50

// Event is something the session emits to its subscribers.
type Event struct {
	Type    string          `json:"type"`
	GameID  string          `json:"gameId"`
	Payload json.RawMessage `json:"payload"`
	// TargetUser, when set, delivers the event to just that participant —
	// used for MOVE_REJECTED, which only the sender should see.
	TargetUser *uuid.UUID `json:"-"`
}

// Subscriber receives events for one connection.
type Subscriber struct {
	UserID uuid.UUID
	// Spectator connections receive state but cannot act.
	Spectator bool
	Out       chan Event
}

// Player identifies one side.
type Player struct {
	UserID *uuid.UUID
	BotID  *string
	Color  string // "black" | "white"
}

// Config describes a game being created.
type Config struct {
	Black       Player
	White       Player
	Rules       goban.Config
	TimeControl clock.Control
	Mode        string // casual | ranked | friend | tournament | daily
}

// command is a request delivered to the actor goroutine.
type command struct {
	kind   string
	userID uuid.UUID
	seq    int64
	point  *goban.Point
	text   string
	sub    *Subscriber
	reply  chan error
}

// Session is one live game.
type Session struct {
	ID    uuid.UUID
	cfg   Config
	db    *pgxpool.Pool
	hooks Hooks
	owner string

	// Everything below is owned by the actor goroutine.
	state        *goban.State
	clock        *clock.Controller
	dead         map[goban.Point]bool
	confirmed    map[string]bool
	subscribers  map[*Subscriber]struct{}
	lastMoveAt   time.Time
	lastChargeAt time.Time
	lastSeq      map[string]int64
	revision     int64
	ended        bool

	// finished mirrors `ended` for readers outside the actor goroutine (the
	// hub's reaper). `ended` itself stays actor-owned and lock-free.
	finished atomic.Bool

	commands  chan command
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
}

// Hooks let other phases observe game completion without this package
// importing them: E1 updates ratings, E2 collects anti-cheat signals, C4
// uploads the SGF, C5 queues the opponent's push.
type Hooks struct {
	OnGameEnded func(ctx context.Context, gameID uuid.UUID, result Result)
	OnMove      func(ctx context.Context, gameID uuid.UUID, m goban.Move, thinkMillis int)
}

// Result summarises a finished game.
type Result struct {
	Winner    string       `json:"winner"`
	Result    string       `json:"result"`
	EndReason string       `json:"endReason"`
	Score     *goban.Score `json:"score,omitempty"`
}

// New builds a session and starts its actor goroutine.
func New(ctx context.Context, db *pgxpool.Pool, id uuid.UUID, cfg Config, hooks Hooks) *Session {
	s := newSession(db, id, cfg, hooks)
	go s.run(ctx)
	return s
}

func newSession(db *pgxpool.Pool, id uuid.UUID, cfg Config, hooks Hooks) *Session {
	first := "black"
	if cfg.Rules.Handicap > 0 {
		first = "white"
	}
	s := &Session{
		ID: id, cfg: cfg, db: db, hooks: hooks,
		state:        goban.NewGame(cfg.Rules),
		clock:        clock.New(cfg.TimeControl, first),
		dead:         map[goban.Point]bool{},
		confirmed:    map[string]bool{},
		subscribers:  map[*Subscriber]struct{}{},
		lastMoveAt:   time.Now().UTC(),
		lastChargeAt: time.Now().UTC(),
		lastSeq:      map[string]int64{},
		commands:     make(chan command, 64),
		done:         make(chan struct{}),
		stopped:      make(chan struct{}),
	}
	return s
}

// Restore rebuilds a session from Postgres, replaying its moves so the
// in-memory position is derived from the same engine that validated them.
func Restore(ctx context.Context, db *pgxpool.Pool, id uuid.UUID, cfg Config,
	moves []StoredMove, clockState clock.State, hooks Hooks) (*Session, error) {
	s, err := restoreSession(db, id, cfg, moves, clockState, hooks)
	if err != nil {
		return nil, err
	}
	go s.run(ctx)
	return s, nil
}

func restoreSession(db *pgxpool.Pool, id uuid.UUID, cfg Config, moves []StoredMove, clockState clock.State, hooks Hooks) (*Session, error) {
	s := newSession(db, id, cfg, hooks)
	replay := goban.NewGame(cfg.Rules)
	for _, m := range moves {
		if m.MoveNumber != len(replay.History)+1 || (m.Player != "black" && m.Player != "white") ||
			(m.Kind == "place" && (m.Row == nil || m.Col == nil)) ||
			(m.Kind != "place" && m.Kind != "pass" && m.Kind != "resign") {
			return nil, fmt.Errorf("invalid stored move %d", m.MoveNumber)
		}
		// A subsequent move after two passes records that play was resumed.
		if replay.Status == goban.StatusScoring {
			replay.Status, replay.ConsecutivePasses = goban.StatusActive, 0
		}
		if m.Kind == "resign" {
			replay.ToMove = parseColor(m.Player)
		}
		if colorName(replay.ToMove) != m.Player {
			return nil, fmt.Errorf("wrong player at move %d", m.MoveNumber)
		}
		next, _, err := replay.Apply(m.Intent())
		if err != nil {
			return nil, fmt.Errorf("replay move %d: %w", m.MoveNumber, err)
		}
		replay = next
	}
	s.state = replay
	// Restore charges time that passed while the game was not in memory.
	now := time.Now().UTC()
	restoreAt := now
	if replay.Status != goban.StatusActive {
		restoreAt = clockState.UpdatedAt
	}
	s.clock = clock.Restore(clockState, restoreAt)
	s.lastMoveAt = clockState.UpdatedAt
	if s.lastMoveAt.IsZero() {
		s.lastMoveAt = now
	}
	s.lastChargeAt = now
	return s, nil
}

// StoredMove is a persisted move row.
type StoredMove struct {
	MoveNumber int
	Player     string
	Kind       string
	Row, Col   *int
}

// Intent converts a stored move back into an engine intent.
func (m StoredMove) Intent() goban.Intent {
	switch m.Kind {
	case "pass":
		return goban.PassIntent()
	case "resign":
		return goban.ResignIntent()
	default:
		return goban.PlaceAt(goban.Point{Row: *m.Row, Col: *m.Col})
	}
}

// Finished reports whether the game has ended. Safe from any goroutine.
func (s *Session) Finished() bool { return s.finished.Load() }

// Close stops the actor.
func (s *Session) Close() { s.closeOnce.Do(func() { close(s.done) }) }

func (s *Session) Closed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// --- public API: every one of these hands work to the actor goroutine ---

// Subscribe attaches a connection.
func (s *Session) Subscribe(sub *Subscriber) error {
	return s.send(command{kind: "subscribe", sub: sub})
}

// Unsubscribe detaches a connection.
func (s *Session) Unsubscribe(sub *Subscriber) error {
	return s.send(command{kind: "unsubscribe", sub: sub})
}

// PlayMove submits a stone placement.
func (s *Session) PlayMove(userID uuid.UUID, seq int64, p goban.Point) error {
	return s.send(command{kind: "move", userID: userID, seq: seq, point: &p})
}

// Pass submits a pass.
func (s *Session) Pass(userID uuid.UUID, seq int64) error {
	return s.send(command{kind: "pass", userID: userID, seq: seq})
}

// Resign submits a resignation.
func (s *Session) Resign(userID uuid.UUID, seq int64) error {
	return s.send(command{kind: "resign", userID: userID, seq: seq})
}

// MarkDead toggles a group's dead status during scoring.
func (s *Session) MarkDead(userID uuid.UUID, seq int64, p goban.Point) error {
	return s.send(command{kind: "mark_dead", userID: userID, seq: seq, point: &p})
}

// ConfirmScore accepts the current dead-stone marking.
func (s *Session) ConfirmScore(userID uuid.UUID, seq int64) error {
	return s.send(command{kind: "confirm_score", userID: userID, seq: seq})
}

// DisputeScore rejects the marking and resumes play.
func (s *Session) DisputeScore(userID uuid.UUID, seq int64) error {
	return s.send(command{kind: "dispute_score", userID: userID, seq: seq})
}

// Chat broadcasts a chat line.
func (s *Session) Chat(userID uuid.UUID, text string) error {
	return s.send(command{kind: "chat", userID: userID, text: text})
}

// Tick charges elapsed time and ends the game if someone flagged. The
// timeout watcher calls this.
func (s *Session) Tick() error { return s.send(command{kind: "tick"}) }

var errSessionClosed = errors.New("game: session closed")

func (s *Session) send(c command) error {
	if s.Closed() {
		return errSessionClosed
	}
	c.reply = make(chan error, 1)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-s.done:
		return errSessionClosed
	case s.commands <- c:
	case <-timer.C:
		return errors.New("game: session busy")
	}
	select {
	case err := <-c.reply:
		return err
	case <-s.done:
		return errSessionClosed
	case <-timer.C:
		return errors.New("game: command timed out")
	}
}

// --- the actor ---

func (s *Session) run(ctx context.Context) {
	defer func() {
		s.Close()
		defer close(s.stopped)
		for sub := range s.subscribers {
			close(sub.Out)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case c := <-s.commands:
			opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			s.handle(opCtx, c)
			cancel()
			c.reply <- nil
		}
	}
}

func (s *Session) handle(ctx context.Context, c command) {
	switch c.kind {
	case "subscribe":
		s.subscribers[c.sub] = struct{}{}
		// A joining connection immediately gets enough state to render the
		// board, whether it is a first join or a reconnect.
		s.emitTo(c.sub, "GAME_SNAPSHOT", s.snapshot())
	case "unsubscribe":
		if _, ok := s.subscribers[c.sub]; ok {
			delete(s.subscribers, c.sub)
			close(c.sub.Out)
		}
	case "move":
		s.applyMove(ctx, c, goban.PlaceAt(*c.point))
	case "pass":
		s.applyMove(ctx, c, goban.PassIntent())
	case "resign":
		s.applyMove(ctx, c, goban.ResignIntent())
	case "mark_dead":
		s.markDead(ctx, c)
	case "confirm_score":
		s.confirmScore(ctx, c)
	case "dispute_score":
		s.disputeScore(ctx, c)
	case "chat":
		s.emit("CHAT_RECEIVED", map[string]any{
			"userId": c.userID, "text": c.text, "at": time.Now().UTC(),
		})
	case "tick":
		s.tick(ctx)
	}
}

// colorOf returns which side a user plays, or "" if they are only watching.
func (s *Session) colorOf(userID uuid.UUID) string {
	if s.cfg.Black.UserID != nil && *s.cfg.Black.UserID == userID {
		return "black"
	}
	if s.cfg.White.UserID != nil && *s.cfg.White.UserID == userID {
		return "white"
	}
	return ""
}

func (s *Session) applyMove(ctx context.Context, c command, intent goban.Intent) {
	color := s.colorOf(c.userID)
	if color == "" {
		s.reject(c, "not_a_player")
		return
	}
	if s.ended {
		s.reject(c, string(goban.RejGameNotActive))
		return
	}
	if !s.validSeq(c) {
		return
	}
	if s.state.Status != goban.StatusActive && intent.Kind != goban.Resign {
		s.reject(c, string(goban.RejGameNotActive))
		return
	}
	// Turn order is checked before the engine so an out-of-turn move gets the
	// specific reason rather than a generic rejection.
	if intent.Kind != goban.Resign && color != colorName(s.state.ToMove) {
		s.reject(c, string(goban.RejNotYourTurn))
		return
	}

	now := time.Now().UTC()
	thinkMillis := int(now.Sub(s.lastMoveAt).Milliseconds())

	// Charge the thinking time before validating: a move that arrives after
	// the flag has fallen must not be accepted.
	if flagged := s.charge(now); flagged != "" {
		s.endGame(ctx, Result{
			Winner:    other(flagged),
			Result:    winnerLetter(other(flagged)) + "+T",
			EndReason: "timeout",
		})
		return
	}

	base := s.state
	if intent.Kind == goban.Resign {
		base = base.Clone()
		base.ToMove, base.Status = parseColor(color), goban.StatusActive
	}
	next, move, err := base.Apply(intent)
	if err != nil {
		var rej *goban.RejectedError
		if errors.As(err, &rej) {
			s.reject(c, string(rej.Reason))
			return
		}
		s.reject(c, "internal_error")
		return
	}

	previous, previousClock := s.state, *s.clock
	previousMoveAt, previousSeq := s.lastMoveAt, s.lastSeq[color]
	s.state = next
	if intent.Kind != goban.Resign {
		s.clock.OnMovePlayed(color)
	}
	s.lastMoveAt = now
	s.lastSeq[color] = c.seq
	var result *Result
	if next.Status == goban.StatusResigned {
		result = &Result{Winner: other(color), Result: winnerLetter(other(color)) + "+R", EndReason: "resign"}
		s.state.Status = goban.StatusComplete
	}
	if err := s.persist(ctx, move, thinkMillis, result); err != nil {
		s.state, *s.clock = previous, previousClock
		s.lastMoveAt, s.lastSeq[color] = previousMoveAt, previousSeq
		s.persistenceFailed(c, err)
		return
	}
	if s.hooks.OnMove != nil {
		s.hooks.OnMove(ctx, s.ID, *move, thinkMillis)
	}

	s.emitTo2(c, "MOVE_ACCEPTED", map[string]any{"seq": c.seq, "moveNumber": move.Number})
	s.emit("MOVE_PLAYED", map[string]any{
		"move":      move,
		"stateHash": s.state.StateHash(),
		"toMove":    colorName(s.state.ToMove),
	})
	s.emitClock()

	if s.state.Status == goban.StatusScoring {
		s.emit("SCORING_STARTED", s.scorePayload())
	}
	if result != nil {
		s.publishResult(ctx, *result)
	}
}

func (s *Session) reject(c command, reason string) {
	payload, _ := json.Marshal(map[string]any{"seq": c.seq, "reason": reason})
	uid := c.userID
	ev := Event{Type: "MOVE_REJECTED", GameID: s.ID.String(), Payload: payload, TargetUser: &uid}
	for sub := range s.subscribers {
		if sub.UserID == uid {
			s.deliver(sub, ev)
		}
	}
}

// markDead toggles the whole group containing p, since players think in
// groups rather than individual stones.
func (s *Session) markDead(ctx context.Context, c command) {
	if s.state.Status != goban.StatusScoring {
		s.reject(c, "not_scoring")
		return
	}
	if s.colorOf(c.userID) == "" {
		s.reject(c, "not_a_player")
		return
	}
	if c.point == nil || !s.state.Board.InBounds(*c.point) {
		s.reject(c, "out_of_bounds")
		return
	}
	if !s.validSeq(c) {
		return
	}
	if s.state.Board.At(*c.point) == goban.Empty {
		s.reject(c, "empty_point")
		return
	}
	previousDead, previousConfirmed := s.deadPoints(), s.confirmed
	color, previousSeq := s.colorOf(c.userID), s.lastSeq[s.colorOf(c.userID)]
	group := goban.FindGroup(s.state.Board, *c.point)
	nowDead := !s.dead[group.Stones[0]]
	for _, st := range group.Stones {
		if nowDead {
			s.dead[st] = true
		} else {
			delete(s.dead, st)
		}
	}
	// Any change to the marking invalidates both players' confirmations.
	s.confirmed = map[string]bool{}
	s.lastSeq[color] = c.seq
	if err := s.persist(ctx, nil, 0, nil); err != nil {
		s.dead = map[goban.Point]bool{}
		for _, p := range previousDead {
			s.dead[p] = true
		}
		s.confirmed, s.lastSeq[color] = previousConfirmed, previousSeq
		s.persistenceFailed(c, err)
		return
	}
	s.emit("SCORE_UPDATED", s.scorePayload())
}

func (s *Session) confirmScore(ctx context.Context, c command) {
	color := s.colorOf(c.userID)
	if color == "" || s.state.Status != goban.StatusScoring {
		s.reject(c, "not_scoring")
		return
	}
	if !s.validSeq(c) {
		return
	}
	previousConfirmed, previousSeq := s.confirmed[color], s.lastSeq[color]
	s.confirmed[color], s.lastSeq[color] = true, c.seq
	if !s.confirmed["black"] || !s.confirmed["white"] {
		if err := s.persist(ctx, nil, 0, nil); err != nil {
			s.confirmed[color], s.lastSeq[color] = previousConfirmed, previousSeq
			s.persistenceFailed(c, err)
			return
		}
		s.emit("SCORE_UPDATED", s.scorePayload())
		return
	}
	score := goban.ScorePosition(s.state, s.deadPoints())
	s.endGame(ctx, Result{
		Winner: score.Winner(), Result: score.Result(),
		EndReason: "score", Score: &score,
	})
}

// disputeScore returns the game to play, which is how two players who cannot
// agree on life and death settle it: by playing it out.
func (s *Session) disputeScore(ctx context.Context, c command) {
	if s.colorOf(c.userID) == "" || s.state.Status != goban.StatusScoring {
		s.reject(c, "not_scoring")
		return
	}
	if !s.validSeq(c) {
		return
	}
	previous, previousDead, previousConfirmed := s.state, s.dead, s.confirmed
	color := s.colorOf(c.userID)
	previousSeq, previousCharge, previousMove := s.lastSeq[color], s.lastChargeAt, s.lastMoveAt
	s.state = s.state.Clone()
	s.state.Status = goban.StatusActive
	s.state.ConsecutivePasses = 0
	s.dead = map[goban.Point]bool{}
	s.confirmed = map[string]bool{}
	s.lastMoveAt = time.Now().UTC()
	s.lastChargeAt = s.lastMoveAt
	s.lastSeq[color] = c.seq
	if err := s.persist(ctx, nil, 0, nil); err != nil {
		s.state, s.dead, s.confirmed = previous, previousDead, previousConfirmed
		s.lastSeq[color], s.lastChargeAt, s.lastMoveAt = previousSeq, previousCharge, previousMove
		s.persistenceFailed(c, err)
		return
	}
	s.emit("GAME_SNAPSHOT", s.snapshot())
}

func (s *Session) tick(ctx context.Context) {
	if s.ended || s.state.Status != goban.StatusActive {
		return
	}
	now := time.Now().UTC()
	// Tick is idempotent in effect: it charges only what has elapsed since
	// the last charge, so calling it more often changes nothing.
	if flagged := s.charge(now); flagged != "" {
		s.endGame(ctx, Result{
			Winner:    other(flagged),
			Result:    winnerLetter(other(flagged)) + "+T",
			EndReason: "timeout",
		})
		return
	}
	s.emitClock()
}

func (s *Session) endGame(ctx context.Context, r Result) {
	if s.ended {
		return
	}
	previous := s.state.Status
	s.state.Status = goban.StatusComplete
	if err := s.persist(ctx, nil, 0, &r); err != nil {
		s.state.Status = previous
		s.persistenceFailed(command{}, err)
		return
	}
	s.publishResult(ctx, r)
}

func (s *Session) publishResult(ctx context.Context, r Result) {
	s.ended = true
	s.finished.Store(true)
	s.emit("GAME_ENDED", r)
	if s.hooks.OnGameEnded != nil {
		s.hooks.OnGameEnded(ctx, s.ID, r)
	}
}

func (s *Session) charge(now time.Time) string {
	if s.state.Status != goban.StatusActive {
		return ""
	}
	elapsed := int(now.Sub(s.lastChargeAt).Milliseconds())
	s.lastChargeAt = now
	// Restoring an already-expired clock must still end the game at a zero delta.
	if s.clock.Black.Flagged {
		return "black"
	}
	if s.clock.White.Flagged {
		return "white"
	}
	return s.clock.Tick(elapsed)
}

func (s *Session) validSeq(c command) bool {
	if c.seq <= s.lastSeq[s.colorOf(c.userID)] {
		s.reject(c, "duplicate_or_stale_seq")
		return false
	}
	return true
}

func parseColor(color string) goban.Color {
	if color == "black" {
		return goban.Black
	}
	return goban.White
}

// --- event emission ---

func (s *Session) snapshot() map[string]any {
	history := s.state.History
	if len(history) > snapshotMoves {
		history = history[len(history)-snapshotMoves:]
	}
	return map[string]any{
		"gameId":      s.ID,
		"status":      s.state.Status,
		"toMove":      colorName(s.state.ToMove),
		"moveNumber":  s.state.MoveNumber,
		"stateHash":   s.state.StateHash(),
		"board":       boardRows(s.state),
		"recentMoves": history,
		"captures": map[string]int{
			"black": s.state.CapturesByBlack, "white": s.state.CapturesByWhite,
		},
		"clock":      s.clock.Export(time.Now().UTC()),
		"config":     s.cfg.Rules,
		"deadStones": s.deadPoints(),
		"lastSeq":    s.lastSeq,
		"confirmed":  s.confirmed,
	}
}

// boardRows renders the position as one string per row, which is far more
// compact over the wire than an array of 361 objects.
func boardRows(st *goban.State) []string {
	size := st.Board.Size()
	rows := make([]string, size)
	for r := 0; r < size; r++ {
		line := make([]byte, size)
		for c := 0; c < size; c++ {
			switch st.Board.AtRC(r, c) {
			case goban.Black:
				line[c] = 'b'
			case goban.White:
				line[c] = 'w'
			default:
				line[c] = '.'
			}
		}
		rows[r] = string(line)
	}
	return rows
}

func (s *Session) scorePayload() map[string]any {
	score := goban.ScorePosition(s.state, s.deadPoints())
	return map[string]any{
		"score":      score,
		"result":     score.Result(),
		"deadStones": s.deadPoints(),
		"confirmed":  s.confirmed,
	}
}

func (s *Session) deadPoints() []goban.Point {
	out := make([]goban.Point, 0, len(s.dead))
	for p := range s.dead {
		out = append(out, p)
	}
	// Deterministic order keeps payloads stable between emissions.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && (out[j].Row < out[j-1].Row ||
			(out[j].Row == out[j-1].Row && out[j].Col < out[j-1].Col)); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func (s *Session) emitClock() {
	s.emit("CLOCK_UPDATE", s.clock.Export(time.Now().UTC()))
}

func (s *Session) emit(t string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ev := Event{Type: t, GameID: s.ID.String(), Payload: raw}
	for sub := range s.subscribers {
		s.deliver(sub, ev)
	}
}

func (s *Session) emitTo(sub *Subscriber, t string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.deliver(sub, Event{Type: t, GameID: s.ID.String(), Payload: raw})
}

func (s *Session) emitTo2(c command, t string, payload any) {
	raw, _ := json.Marshal(payload)
	ev := Event{Type: t, GameID: s.ID.String(), Payload: raw}
	for sub := range s.subscribers {
		if sub.UserID == c.userID {
			s.deliver(sub, ev)
		}
	}
}

// deliver never blocks the actor: a connection whose buffer is full is
// dropped rather than stalling the whole game for everyone else.
func (s *Session) deliver(sub *Subscriber, ev Event) {
	select {
	case sub.Out <- ev:
	default:
		delete(s.subscribers, sub)
		close(sub.Out)
	}
}

func colorName(c goban.Color) string {
	if c == goban.Black {
		return "black"
	}
	return "white"
}

func other(color string) string {
	if color == "black" {
		return "white"
	}
	return "black"
}

func winnerLetter(color string) string {
	if color == "black" {
		return "B"
	}
	return "W"
}
