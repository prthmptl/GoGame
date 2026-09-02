package api

import (
	"log/slog"
	"net/http"

	"github.com/prathpatel/gogame-backend/internal/logging"
)

// logging returns the request-scoped structured logger.
func logging_(r *http.Request) *slog.Logger { return logging.FromContext(r.Context()) }
