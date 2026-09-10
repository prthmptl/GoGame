package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/auth"
)

type ctxKeyUser struct{}

// authenticated wraps a handler so it only runs for a caller with a valid
// access token, and stores the caller's id on the context.
func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "authorization required")
			return
		}
		claims, err := s.auth.ParseAccessToken(raw)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token", "token is not valid")
			return
		}
		userID, err := claims.UserID()
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token", "token subject is malformed")
			return
		}
		if s.anticheat != nil {
			restrictions, err := s.anticheat.Restrictions(r.Context(), userID)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			for _, restriction := range restrictions {
				if restriction.Kind == "suspend" {
					writeError(w, http.StatusForbidden, "suspended", "account suspended")
					return
				}
			}
		}
		ctx := context.WithValue(r.Context(), ctxKeyUser{}, principal{ID: userID, IsGuest: claims.IsGuest})
		next(w, r.WithContext(ctx))
	}
}

// principal is the authenticated caller.
type principal struct {
	ID      uuid.UUID
	IsGuest bool
}

// callerFrom returns the authenticated principal. Only valid inside a handler
// wrapped by authenticated.
func callerFrom(ctx context.Context) principal {
	p, _ := ctx.Value(ctxKeyUser{}).(principal)
	return p
}

// requireFullAccount rejects guests. Ranked play, chat and social features
// need a recoverable account behind them.
func (s *Server) requireFullAccount(next http.HandlerFunc) http.HandlerFunc {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request) {
		if callerFrom(r.Context()).IsGuest {
			writeError(w, http.StatusForbidden, "guest_not_allowed",
				"sign in with Google to use this feature")
			return
		}
		next(w, r)
	})
}

// unused keeps auth imported when only the type is referenced.
var _ = auth.ErrInvalidToken
