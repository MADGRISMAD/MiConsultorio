// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	JWTSecret      []byte
	CookieSecure   bool
	SessionTTL     time.Duration
	AllowedOrigins []string // extra origins allowed to make state-changing requests
	StaticDir      string   // optional: serve the built frontend from this directory

	// Optional first-run setup: when set and the database has no clinics, one is created.
	AdminEmail, AdminUsername, AdminPassword, ClinicName, ClinicKind string
}

func Load() (*Config, error) {
	c := &Config{
		Addr:         env("ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		CookieSecure: env("COOKIE_SECURE", "true") != "false",
		SessionTTL:   12 * time.Hour,
		StaticDir:    os.Getenv("STATIC_DIR"),

		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminUsername: env("ADMIN_USERNAME", "admin"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		ClinicName:    env("CLINIC_NAME", "Mi Clínica"),
		ClinicKind:    env("CLINIC_KIND", "GENERAL_MEDICAL"),
	}
	if c.StaticDir == "" {
		for _, d := range []string{"../frontend/build", "frontend/build"} {
			if _, err := os.Stat(d + "/index.html"); err == nil {
				c.StaticDir = d
				break
			}
		}
	}
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}
	c.JWTSecret = []byte(secret)
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, strings.TrimRight(o, "/"))
		}
	}
	if v := os.Getenv("SESSION_TTL_HOURS"); v != "" {
		var h int
		if _, err := fmt.Sscanf(v, "%d", &h); err != nil || h <= 0 {
			return nil, errors.New("SESSION_TTL_HOURS must be a positive integer")
		}
		c.SessionTTL = time.Duration(h) * time.Hour
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
