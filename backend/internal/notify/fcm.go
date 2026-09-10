package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// FCM delivers via Firebase Cloud Messaging HTTP v1. The same endpoint serves
// Android, web and — through Firebase's APNs bridge — iOS, which is why C5's
// "FCM / APNs integration" needs only one sender in practice.
type FCM struct {
	ProjectID string
	http      *http.Client
	mu        sync.Mutex
	tokenSrc  oauth2.TokenSource
}

// NewFCMFromEnv builds a sender from GOOGLE_APPLICATION_CREDENTIALS and
// FCM_PROJECT_ID, returning nil when push is not configured so the worker
// simply skips delivery in environments without credentials.
func NewFCMFromEnv() *FCM {
	projectID := os.Getenv("FCM_PROJECT_ID")
	if projectID == "" {
		return nil
	}
	return &FCM{ProjectID: projectID, http: &http.Client{Timeout: 15 * time.Second}}
}

func (f *FCM) source(ctx context.Context) (oauth2.TokenSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tokenSrc != nil {
		return f.tokenSrc, nil
	}
	// OAuth retains this context for future refreshes. A delivery batch's
	// cancelled context would otherwise break all subsequent token refreshes.
	// The dedicated HTTP client still bounds every credential request.
	credentialCtx := context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, f.http)
	creds, err := google.FindDefaultCredentials(credentialCtx,
		"https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, fmt.Errorf("load FCM credentials: %w", err)
	}
	f.tokenSrc = creds.TokenSource
	return f.tokenSrc, nil
}

// Send delivers one notification to one device token.
func (f *FCM) Send(ctx context.Context, token, platform string, n Notification) error {
	src, err := f.source(ctx)
	if err != nil {
		return err
	}
	tok, err := src.Token()
	if err != nil {
		return fmt.Errorf("fcm token: %w", err)
	}

	msg := map[string]any{
		"message": map[string]any{
			"token":        token,
			"notification": map[string]string{"title": n.Title, "body": n.Body},
			"data":         n.Data,
			"android": map[string]any{
				"priority": "high",
				// Collapsing means a second "your turn" for the same game
				// replaces the first rather than stacking on the lock screen.
				"collapse_key": n.CollapseKey,
			},
			"apns": map[string]any{
				"headers": map[string]string{
					"apns-collapse-id": n.CollapseKey,
					"apns-priority":    "10",
				},
			},
		},
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", f.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.http.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		return nil
	}
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// 404 UNREGISTERED and 400 INVALID_ARGUMENT mean the token will never
	// work again — disable it instead of retrying for hours.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return &PermanentError{Reason: fmt.Sprintf("%s: %s", resp.Status, payload)}
	}
	return fmt.Errorf("fcm send %s: %s", resp.Status, payload)
}

// NoopSender drops notifications. Only use it explicitly in tests, not without
// push credentials.
type NoopSender struct{}

// Send does nothing successfully.
func (NoopSender) Send(context.Context, string, string, Notification) error { return nil }
