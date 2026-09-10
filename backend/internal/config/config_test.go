package config

import (
	"strings"
	"testing"
)

func baseline(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PORT", "JWT_SECRET", "ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "SHUTDOWN_GRACE", "TRUSTED_PROXY_CIDRS", "S3_BUCKET", "S3_ENDPOINT", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
}

func TestInvalidConfigurationFailsInsteadOfSilentlyDefaulting(t *testing.T) {
	for _, pair := range [][2]string{{"PORT", "banana"}, {"PORT", "65536"}, {"ACCESS_TOKEN_TTL", "-1m"}, {"REFRESH_TOKEN_TTL", "bad"}, {"SHUTDOWN_GRACE", "0s"}, {"TRUSTED_PROXY_CIDRS", "all"}} {
		t.Run(pair[0]+pair[1], func(t *testing.T) {
			baseline(t)
			t.Setenv(pair[0], pair[1])
			if _, err := Load(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
func TestProductionRequiresDurableStorageAndRejectsDevelopmentSecret(t *testing.T) {
	baseline(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("GOOGLE_CLIENT_IDS", "test-client")
	t.Setenv("JWT_SECRET", "dev-insecure-signing-key-do-not-use-in-prod")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "S3_BUCKET") || !strings.Contains(err.Error(), "development JWT_SECRET") {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", strings.Repeat("s", 40))
	t.Setenv("S3_BUCKET", "test")
	t.Setenv("S3_ENDPOINT", "https://storage.example.test")
	t.Setenv("S3_ACCESS_KEY_ID", "key")
	t.Setenv("S3_SECRET_ACCESS_KEY", "secret")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
