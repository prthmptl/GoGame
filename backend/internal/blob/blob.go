// Package blob abstracts S3-compatible object storage for SGF bodies (C4).
package blob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// Store reads and writes objects.
type Store interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	// SignedURL returns a time-limited direct-download URL, so SGF bodies are
	// served by the CDN rather than proxied through the API.
	SignedURL(key string, ttl time.Duration) (string, error)
}

// Memory is an in-process Store for tests and local development.
type Memory struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory { return &Memory{objects: map[string][]byte{}} }

func (m *Memory) Put(_ context.Context, key string, body []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := make([]byte, len(body))
	copy(buf, body)
	m.objects[key] = buf
	return nil
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("blob: %s not found", key)
	}
	return b, nil
}

func (m *Memory) SignedURL(key string, _ time.Duration) (string, error) {
	return "memory://" + key, nil
}

// S3 talks to any S3-compatible endpoint (AWS, R2, Backblaze, MinIO) using
// SigV4 signed requests. Implemented directly rather than pulling the AWS SDK:
// PUT, GET and presign are all we need, and the SDK is a large dependency for
// three calls.
type S3 struct {
	Endpoint  string // https://s3.us-east-1.amazonaws.com or https://<acct>.r2.cloudflarestorage.com
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	// PublicBaseURL, when set, is the CDN origin used for signed URLs.
	PublicBaseURL string
	HTTP          *http.Client
}

// NewS3FromEnv builds an S3 store from the standard environment variables,
// returning nil when object storage is not configured.
func NewS3FromEnv() *S3 {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil
	}
	return &S3{
		Endpoint:      strings.TrimSuffix(os.Getenv("S3_ENDPOINT"), "/"),
		Region:        envOr("S3_REGION", "auto"),
		Bucket:        bucket,
		AccessKey:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:     os.Getenv("S3_SECRET_ACCESS_KEY"),
		PublicBaseURL: strings.TrimSuffix(os.Getenv("S3_PUBLIC_BASE_URL"), "/"),
		HTTP:          &http.Client{Timeout: 30 * time.Second},
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func (s *S3) objectURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", s.Endpoint, s.Bucket, strings.TrimPrefix(key, "/"))
}

func (s *S3) Put(ctx context.Context, key string, body []byte, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(key), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if err := s.sign(req, body); err != nil {
		return err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("s3 put %s: %s: %s", key, resp.Status, msg)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(key), nil)
	if err != nil {
		return nil, err
	}
	if err := s.sign(req, nil); err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 get %s: %s", key, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// SignedURL returns a presigned GET URL valid for ttl.
func (s *S3) SignedURL(key string, ttl time.Duration) (string, error) {
	if s.PublicBaseURL != "" {
		// Public CDN bucket: no signature needed.
		return s.PublicBaseURL + "/" + strings.TrimPrefix(key, "/"), nil
	}
	now := time.Now().UTC()
	stamp := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", date, s.Region)

	u, err := url.Parse(s.objectURL(key))
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", s.AccessKey+"/"+scope)
	q.Set("X-Amz-Date", stamp)
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", int(ttl.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")
	u.RawQuery = q.Encode()

	canonical := strings.Join([]string{
		http.MethodGet, u.EscapedPath(), u.RawQuery,
		"host:" + u.Host + "\n", "host", "UNSIGNED-PAYLOAD",
	}, "\n")
	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", stamp, scope, sha256Hex([]byte(canonical)),
	}, "\n")

	sig := hex.EncodeToString(hmacSHA256(s.signingKey(date), []byte(toSign)))
	u.RawQuery += "&X-Amz-Signature=" + sig
	return u.String(), nil
}

// sign applies SigV4 to a request.
func (s *S3) sign(req *http.Request, body []byte) error {
	now := time.Now().UTC()
	stamp := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256Hex(body)

	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, payloadHash, stamp)
	canonical := strings.Join([]string{
		req.Method, req.URL.EscapedPath(), req.URL.RawQuery,
		canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	scope := fmt.Sprintf("%s/%s/s3/aws4_request", date, s.Region)
	toSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", stamp, scope, sha256Hex([]byte(canonical)),
	}, "\n")
	sig := hex.EncodeToString(hmacSHA256(s.signingKey(date), []byte(toSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.AccessKey, scope, signedHeaders, sig))
	return nil
}

func (s *S3) signingKey(date string) []byte {
	k := hmacSHA256([]byte("AWS4"+s.SecretKey), []byte(date))
	k = hmacSHA256(k, []byte(s.Region))
	k = hmacSHA256(k, []byte("s3"))
	return hmacSHA256(k, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// GameKey is the canonical object key for a game's SGF. Sharding by the first
// two hex characters of the id keeps any single storage prefix from becoming
// a hotspot.
func GameKey(gameID string) string {
	shard := gameID
	if len(shard) >= 2 {
		shard = shard[:2]
	}
	return path.Join("games", shard, gameID+".sgf")
}
