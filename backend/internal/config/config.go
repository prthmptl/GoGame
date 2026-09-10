// Package config loads process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env identifies the deployment environment.
type Env string

const (
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "production"
)

// Config is the fully-resolved configuration for the API server and worker.
type Config struct {
	Env  Env
	Port int

	DatabaseURL string
	RedisURL    string

	// AccessTokenTTL is deliberately short: refresh rotation is what keeps a
	// session alive, so a stolen access token stops working quickly.
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// JWTSecret signs access tokens (HS256). Must be >= 32 bytes outside dev.
	JWTSecret []byte
	JWTIssuer string

	// GoogleClientIDs are the audiences accepted on POST /auth/google. The
	// Android, iOS and web OAuth clients each have their own ID.
	GoogleClientIDs []string

	ShutdownGracePeriod time.Duration
}

// Load reads configuration from the environment, applying dev-friendly
// defaults. It reports every problem it finds rather than failing on the
// first, so a misconfigured deploy surfaces all of them at once.
func Load() (*Config, error) {
	c := &Config{
		Env:                 Env(getenv("APP_ENV", string(EnvDev))),
		Port:                getenvInt("PORT", 8080),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RedisURL:            os.Getenv("REDIS_URL"),
		AccessTokenTTL:      getenvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getenvDuration("REFRESH_TOKEN_TTL", 60*24*time.Hour),
		JWTSecret:           []byte(os.Getenv("JWT_SECRET")),
		JWTIssuer:           getenv("JWT_ISSUER", "gogame"),
		ShutdownGracePeriod: getenvDuration("SHUTDOWN_GRACE", 20*time.Second),
	}
	if raw := os.Getenv("GOOGLE_CLIENT_IDS"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			if id = strings.TrimSpace(id); id != "" {
				c.GoogleClientIDs = append(c.GoogleClientIDs, id)
			}
		}
	}

	var problems []string
	if c.Port < 1 || c.Port > 65535 {
		problems = append(problems, "PORT must be between 1 and 65535")
	}
	if raw := os.Getenv("PORT"); raw != "" {
		if _, err := strconv.Atoi(raw); err != nil {
			problems = append(problems, "PORT must be an integer")
		}
	}
	for _, key := range []string{"ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "SHUTDOWN_GRACE"} {
		if raw := os.Getenv(key); raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				problems = append(problems, key+" must be a positive duration")
			}
		}
	}
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(raw) != "" {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(raw)); err != nil {
				problems = append(problems, "invalid TRUSTED_PROXY_CIDRS")
			}
		}
	}
	if c.Env != EnvDev {
		if string(c.JWTSecret) == "dev-insecure-signing-key-do-not-use-in-prod" {
			problems = append(problems, "development JWT_SECRET is forbidden outside dev")
		}
		for _, key := range []string{"S3_BUCKET", "S3_ENDPOINT", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
			if strings.TrimSpace(os.Getenv(key)) == "" {
				problems = append(problems, key+" is required outside dev")
			}
		}
		endpoint, err := url.Parse(os.Getenv("S3_ENDPOINT"))
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			problems = append(problems, "S3_ENDPOINT must be an HTTPS URL outside dev")
		}
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		problems = append(problems, "REDIS_URL is required")
	}
	switch c.Env {
	case EnvDev, EnvStaging, EnvProd:
	default:
		problems = append(problems, fmt.Sprintf("APP_ENV %q must be dev, staging or production", c.Env))
	}
	if len(c.JWTSecret) == 0 {
		if c.Env == EnvDev {
			// Deterministic dev secret keeps tokens valid across restarts.
			c.JWTSecret = []byte("dev-insecure-signing-key-do-not-use-in-prod")
		} else {
			problems = append(problems, "JWT_SECRET is required outside dev")
		}
	} else if len(c.JWTSecret) < 32 && c.Env != EnvDev {
		problems = append(problems, "JWT_SECRET must be at least 32 bytes")
	}
	if len(c.GoogleClientIDs) == 0 && c.Env == EnvProd {
		problems = append(problems, "GOOGLE_CLIENT_IDS is required in production")
	}
	if len(problems) > 0 {
		return nil, errors.New("config: " + strings.Join(problems, "; "))
	}
	return c, nil
}

func (c *Config) IsProd() bool { return c.Env == EnvProd }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
