package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/chat"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// D1's timings: heartbeat every 20s, disconnect after 60s of silence.
const (
	heartbeatInterval = 20 * time.Second
	idleTimeout       = 60 * time.Second
	writeTimeout      = 10 * time.Second
	// outboundBuffer sizes each connection's event queue. A slow client that
	// fills it is disconnected so it can reconnect for a fresh snapshot.
	outboundBuffer = 64
)

// Server upgrades HTTP connections and runs the D1 protocol.
type Server struct {
	auth *auth.Service
	hub  *game.Hub
	log  *slog.Logger
	// AllowedOrigins gates the CORS check on the upgrade. Empty means
	// same-origin only, which is right for the mobile app.
	AllowedOrigins []string
	Chat           *chat.Service
	Archive        *archive.Service
	Anticheat      *anticheat.Service
}

// NewServer builds the WebSocket handler.
func NewServer(a *auth.Service, h *game.Hub, lg *slog.Logger, origins []string) *Server {
	return &Server{auth: a, hub: h, log: lg, AllowedOrigins: origins}
}

// conn is one client connection.
type conn struct {
	ws            *websocket.Conn
	userID        uuid.UUID
	isGuest       bool
	sub           *game.Subscriber
	session       *game.Session
	log           *slog.Logger
	expiresAt     time.Time
	forwardCancel context.CancelFunc
}

// Handle serves GET /ws.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.AllowedOrigins,
	})
	if err != nil {
		s.log.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(16 << 10)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// D1 allows the token in the URL query or the first frame. The query form
	// exists because some mobile WebSocket clients cannot set headers.
	var claims *auth.Claims
	if tok := r.URL.Query().Get("token"); tok != "" {
		claims, err = s.auth.ParseAccessToken(tok)
		if err != nil {
			writeFrame(ctx, c, Envelope{Type: string(MsgError)},
				ErrorPayload{Code: "invalid_token", Message: "token is not valid"})
			_ = c.Close(websocket.StatusPolicyViolation, "invalid token")
			return
		}
	}

	cn := &conn{ws: c, log: s.log}
	if claims != nil {
		if err := cn.authenticate(ctx, claims); err != nil {
			_ = c.Close(websocket.StatusPolicyViolation, "authentication failed")
			return
		}
	}

	go s.pump(ctx, cn)
	s.readLoop(ctx, cn)
}

func (c *conn) authenticate(ctx context.Context, claims *auth.Claims) error {
	id, err := claims.UserID()
	if err != nil {
		return err
	}
	if c.userID != uuid.Nil && c.userID != id {
		return errors.New("identity cannot change on an open connection")
	}
	if claims.ExpiresAt == nil {
		return errors.New("token has no expiry")
	}
	c.expiresAt = claims.ExpiresAt.Time
	c.userID = id
	c.isGuest = claims.IsGuest
	c.log = c.log.With("userId", id)
	return writeFrame(ctx, c.ws, Envelope{Type: string(MsgAuthenticated)},
		map[string]any{"userId": id, "guest": claims.IsGuest})
}

// readLoop consumes client frames until the connection closes or goes idle.
func (s *Server) readLoop(ctx context.Context, c *conn) {
	defer func() {
		if c.forwardCancel != nil {
			c.forwardCancel()
		}
		if c.session != nil && c.sub != nil {
			_ = c.session.Unsubscribe(c.sub)
		}
	}()

	// Bound command work before JSON parsing, authorization queries or actor
	// calls. Normal play and heartbeats consume far less than this allowance.
	tokens, lastRefill := 30.0, time.Now()
	for {
		// A read deadline enforces D1's 60s idle disconnect: heartbeats from a
		// live client reset it, a dead one trips it.
		timeout := idleTimeout
		if !c.expiresAt.IsZero() && time.Until(c.expiresAt) < timeout {
			timeout = time.Until(c.expiresAt)
		}
		readCtx, cancel := context.WithTimeout(ctx, timeout)
		_, data, err := c.ws.Read(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, context.Canceled) {
				c.log.Debug("websocket closed", "error", err)
			}
			return
		}
		now := time.Now()
		tokens = min(30, tokens+now.Sub(lastRefill).Seconds()*10)
		lastRefill = now
		if tokens < 1 {
			s.protoError(ctx, c, 0, "rate_limited", "too many commands")
			c.ws.CloseNow()
			return
		}
		tokens--

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			_ = writeFrame(ctx, c.ws, Envelope{Type: string(MsgError)},
				ErrorPayload{Code: "bad_frame", Message: "frame is not valid JSON"})
			continue
		}
		s.dispatch(ctx, c, env)
	}
}

func (s *Server) dispatch(ctx context.Context, c *conn, env Envelope) {
	if !c.expiresAt.IsZero() && !time.Now().Before(c.expiresAt) {
		s.protoError(ctx, c, env.Seq, "token_expired", "refresh your access token and reconnect")
		c.ws.CloseNow()
		return
	}
	if c.userID != uuid.Nil && s.Anticheat != nil {
		restrictions, err := s.Anticheat.Restrictions(ctx, c.userID)
		if err != nil {
			s.protoError(ctx, c, env.Seq, "unavailable", "authorization unavailable")
			c.ws.CloseNow()
			return
		}
		for _, restriction := range restrictions {
			if restriction.Kind == "suspend" {
				s.protoError(ctx, c, env.Seq, "suspended", "account suspended")
				c.ws.CloseNow()
				return
			}
		}
	}
	// Every message except AUTHENTICATE requires an authenticated connection.
	if c.userID == uuid.Nil && ClientMessageType(env.Type) != MsgAuthenticate {
		_ = writeFrame(ctx, c.ws, Envelope{Type: string(MsgError), Seq: env.Seq},
			ErrorPayload{Code: "unauthenticated", Message: "authenticate first"})
		c.ws.CloseNow()
		return
	}

	if env.GameID != "" && c.session != nil && ClientMessageType(env.Type) != MsgJoinGame && env.GameID != c.session.ID.String() {
		s.protoError(ctx, c, env.Seq, "wrong_game", "join the requested game first")
		return
	}
	switch ClientMessageType(env.Type) {
	case MsgAuthenticate:
		var p AuthenticatePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			s.protoError(ctx, c, env.Seq, "bad_payload", err.Error())
			return
		}
		claims, err := s.auth.ParseAccessToken(p.Token)
		if err != nil {
			s.protoError(ctx, c, env.Seq, "invalid_token", "token is not valid")
			c.ws.CloseNow()
			return
		}
		if err := c.authenticate(ctx, claims); err != nil {
			s.protoError(ctx, c, env.Seq, "auth_failed", "authentication failed")
			c.ws.CloseNow()
		}

	case MsgHeartbeat:
		_ = writeFrame(ctx, c.ws, Envelope{Type: string(MsgPong), Seq: env.Seq}, nil)

	case MsgJoinGame:
		var p JoinGamePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			s.protoError(ctx, c, env.Seq, "bad_payload", err.Error())
			return
		}
		s.joinGame(ctx, c, env.Seq, p.GameID)

	case MsgLeaveGame:
		if c.forwardCancel != nil {
			c.forwardCancel()
			c.forwardCancel = nil
		}
		if c.session != nil && c.sub != nil {
			_ = c.session.Unsubscribe(c.sub)
			c.session, c.sub = nil, nil
		}

	case MsgPlayMove:
		var p PlayMovePayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			s.protoError(ctx, c, env.Seq, "bad_payload", err.Error())
			return
		}
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.PlayMove(c.userID, env.Seq, goban.Point{Row: p.Row, Col: p.Col})
		})

	case MsgPass:
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.Pass(c.userID, env.Seq)
		})

	case MsgResign:
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.Resign(c.userID, env.Seq)
		})

	case MsgMarkDeadGroup:
		var p MarkDeadGroupPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			s.protoError(ctx, c, env.Seq, "bad_payload", err.Error())
			return
		}
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.MarkDead(c.userID, env.Seq, goban.Point{Row: p.Row, Col: p.Col})
		})

	case MsgConfirmScore:
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.ConfirmScore(c.userID, env.Seq)
		})

	case MsgDisputeScore:
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			return g.DisputeScore(c.userID, env.Seq)
		})

	case MsgChatMessage:
		var p ChatPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			s.protoError(ctx, c, env.Seq, "bad_payload", err.Error())
			return
		}
		if c.isGuest {
			// Guests cannot chat: an unrecoverable account is the cheapest
			// possible way to abuse a chat channel.
			s.protoError(ctx, c, env.Seq, "guest_not_allowed", "sign in to chat")
			return
		}
		if len(p.Text) == 0 || len([]rune(p.Text)) > 500 {
			s.protoError(ctx, c, env.Seq, "invalid_chat", "message must be 1-500 characters")
			return
		}
		s.requireGame(ctx, c, env.Seq, func(g *game.Session) error {
			if s.Chat == nil {
				return errors.New("chat unavailable")
			}
			message, err := s.Chat.Post(ctx, g.ID, c.userID, p.Text)
			if err != nil {
				return err
			}
			return g.Chat(c.userID, message.Body)
		})

	default:
		s.protoError(ctx, c, env.Seq, "unknown_type", "unsupported message type "+env.Type)
	}
}

func (s *Server) joinGame(ctx context.Context, c *conn, seq int64, rawID string) {
	gameID, err := uuid.Parse(rawID)
	if err != nil {
		s.protoError(ctx, c, seq, "invalid_game_id", "game id must be a uuid")
		return
	}
	if s.Archive != nil {
		if _, err := s.Archive.Get(ctx, gameID, c.userID); err != nil {
			s.protoError(ctx, c, seq, "game_not_found", "no accessible game")
			return
		}
	}
	sess, err := s.hub.Get(ctx, gameID)
	if err != nil {
		if errors.Is(err, game.ErrOwnedElsewhere) {
			s.protoError(ctx, c, seq, "game_on_another_instance", "reconnect through the game owner")
			return
		}
		s.protoError(ctx, c, seq, "game_not_found", "no such active game")
		return
	}
	// Leave any previous game so one connection never straddles two.
	if c.forwardCancel != nil {
		c.forwardCancel()
	}
	if c.session != nil && c.sub != nil {
		_ = c.session.Unsubscribe(c.sub)
	}
	sub := &game.Subscriber{UserID: c.userID, Out: make(chan game.Event, outboundBuffer)}
	if err := sess.Subscribe(sub); err != nil {
		s.protoError(ctx, c, seq, "join_failed", err.Error())
		return
	}
	c.session, c.sub = sess, sub
	forwardCtx, cancel := context.WithCancel(ctx)
	c.forwardCancel = cancel
	go s.forward(forwardCtx, c, sub)
}

// forward relays session events to the socket.
func (s *Server) forward(ctx context.Context, c *conn, sub *game.Subscriber) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub.Out:
			if !ok {
				if ctx.Err() == nil {
					c.ws.CloseNow()
				}
				return
			}
			env := Envelope{Type: ev.Type, GameID: ev.GameID, Payload: ev.Payload}
			raw, err := json.Marshal(env)
			if err != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err = c.ws.Write(wctx, websocket.MessageText, raw)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// pump sends periodic heartbeats so idle connections stay alive and dead ones
// are noticed.
func (s *Server) pump(ctx context.Context, c *conn) {
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.ws.Ping(wctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) requireGame(ctx context.Context, c *conn, seq int64, fn func(*game.Session) error) {
	if c.session == nil {
		s.protoError(ctx, c, seq, "not_in_game", "join a game first")
		return
	}
	if err := fn(c.session); err != nil {
		s.protoError(ctx, c, seq, "command_failed", err.Error())
	}
}

func (s *Server) protoError(ctx context.Context, c *conn, seq int64, code, msg string) {
	_ = writeFrame(ctx, c.ws, Envelope{Type: string(MsgError), Seq: seq},
		ErrorPayload{Code: code, Message: msg})
}

func writeFrame(ctx context.Context, c *websocket.Conn, env Envelope, payload any) error {
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		env.Payload = raw
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, data)
}
