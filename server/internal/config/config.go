// Package config loads server settings from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Addr         string // public API listen address, e.g. ":8080"
	InternalAddr string // metrics and pprof; never exposed publicly, e.g. "127.0.0.1:6060"
	DatabaseURL  string

	// WebURL is the web app's origin: magic links and OAuth redirects land there.
	WebURL     string
	WebOrigins []string // allowed CORS origins

	// TokenSecret signs access tokens (HMAC-SHA256). At least 32 bytes, base64.
	TokenSecret []byte
	// CookieSecure marks auth cookies Secure. Only disable for plain-HTTP setups;
	// browsers already treat http://localhost as secure.
	CookieSecure bool
	// TrustProxy takes the client IP from X-Forwarded-For. Enable only behind a
	// proxy that sets it, or clients could spoof their IP to dodge rate limits.
	TrustProxy bool

	// MasterKey wraps every user key (envelope encryption). 32 bytes, base64.
	// Losing it makes every stored book unreadable; keep it in a secrets manager.
	MasterKey []byte

	S3     S3Config
	Google GoogleConfig
	Mail   MailConfig
}

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	// CreateBucket creates the bucket at startup (local development only).
	CreateBucket bool
}

// GoogleConfig is optional: without a client ID, Google sign-in is disabled.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
}

func (g GoogleConfig) Enabled() bool { return g.ClientID != "" && g.ClientSecret != "" }

// MailConfig is optional: without a Resend key, emails are written to the log.
type MailConfig struct {
	ResendAPIKey string
	From         string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:         getenv("RIFT_ADDR", ":8080"),
		InternalAddr: getenv("RIFT_INTERNAL_ADDR", "127.0.0.1:6060"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		WebURL:       strings.TrimRight(getenv("RIFT_WEB_URL", "http://localhost:5173"), "/"),
		CookieSecure: getenv("RIFT_COOKIE_SECURE", "true") != "false",
		TrustProxy:   os.Getenv("RIFT_TRUST_PROXY") == "true",
		Google: GoogleConfig{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		},
		S3: S3Config{
			Endpoint:     os.Getenv("S3_ENDPOINT"),
			Region:       getenv("S3_REGION", "auto"),
			Bucket:       getenv("S3_BUCKET", "rift-books"),
			AccessKey:    os.Getenv("S3_ACCESS_KEY"),
			SecretKey:    os.Getenv("S3_SECRET_KEY"),
			CreateBucket: os.Getenv("S3_CREATE_BUCKET") == "true",
		},
		Mail: MailConfig{
			ResendAPIKey: os.Getenv("RESEND_API_KEY"),
			From:         getenv("RIFT_MAIL_FROM", "rift <signin@rift.local>"),
		},
	}
	cfg.WebOrigins = splitList(getenv("RIFT_WEB_ORIGIN", cfg.WebURL))

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if _, err := url.ParseRequestURI(cfg.WebURL); err != nil {
		errs = append(errs, fmt.Errorf("RIFT_WEB_URL: %w", err))
	}
	secret, err := base64.StdEncoding.DecodeString(os.Getenv("RIFT_TOKEN_SECRET"))
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("RIFT_TOKEN_SECRET must be base64: %w", err))
	case len(secret) < 32:
		errs = append(errs, errors.New("RIFT_TOKEN_SECRET must decode to at least 32 bytes"))
	}
	cfg.TokenSecret = secret

	master, err := base64.StdEncoding.DecodeString(os.Getenv("RIFT_MASTER_KEY"))
	if err != nil || len(master) != 32 {
		errs = append(errs, errors.New("RIFT_MASTER_KEY must be 32 bytes, base64-encoded"))
	}
	cfg.MasterKey = master
	if cfg.S3.Endpoint == "" || cfg.S3.AccessKey == "" || cfg.S3.SecretKey == "" {
		errs = append(errs, errors.New("S3_ENDPOINT, S3_ACCESS_KEY and S3_SECRET_KEY are required"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
