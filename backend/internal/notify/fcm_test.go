package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFCMTokenRefreshSurvivesOriginalBatchCancellation(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer endpoint.Close()
	path := filepath.Join(t.TempDir(), "credentials.json")
	raw, _ := json.Marshal(map[string]string{"type": "authorized_user", "client_id": "test", "client_secret": "test", "refresh_token": "test", "token_uri": endpoint.URL})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	f := &FCM{http: &http.Client{Timeout: time.Second}}
	ctx, cancel := context.WithCancel(context.Background())
	source, err := f.source(ctx)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	token, err := source.Token()
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "test-token" {
		t.Fatal("refresh failed")
	}
}
