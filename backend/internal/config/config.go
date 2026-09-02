// Package config loads process configuration from the environment.
package config

import (
	"errors"
	"fmt"
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
