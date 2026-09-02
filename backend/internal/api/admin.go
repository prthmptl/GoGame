package api

import (
	"net/http"
	"os"
	"strings"
)

// isAdmin reports whether the caller may use the moderation endpoints.
//
// Roles are not modelled yet. ADMIN_USER_IDS holds a comma-separated
// allowlist, which is enough for a single trusted operator and explicitly
// not enough for a moderation team: that needs a real roles table before
// E2's review queue is opened up.
func (s *Server) isAdmin(r *http.Request) bool {
	raw := os.Getenv("ADMIN_USER_IDS")
	if raw == "" {
		return false
	}
	caller := callerFrom(r.Context()).ID.String()
	for _, id := range strings.Split(raw, ",") {
		if strings.TrimSpace(id) == caller {
			return true
		}
	}
	return false
}
