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

	// Optional first-run setup. The platform administrator is created when none exists;
	// the sample clinic when the database has no clinics.
	PlatformName, PlatformEmail, PlatformUsername, PlatformPassword string
	ClinicName, ClinicKind                                          string
	ClinicAdminName, ClinicAdminEmail, ClinicAdminUsername          string
	ClinicAdminPassword                                             string
}

func Load() (*Config, error) {
	c := &Config{
		Addr:         env("ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		CookieSecure: env("COOKIE_SECURE", "true") != "false",
		SessionTTL:   12 * time.Hour,
		StaticDir:    os.Getenv("STATIC_DIR"),

		PlatformName:     env("PLATFORM_ADMIN_NAME", "Administrador de Caresia"),
		PlatformEmail:    os.Getenv("PLATFORM_ADMIN_EMAIL"),
		PlatformUsername: env("PLATFORM_ADMIN_USERNAME", "plataforma"),
		PlatformPassword: os.Getenv("PLATFORM_ADMIN_PASSWORD"),

		ClinicName:          env("CLINIC_NAME", "Mi Clínica"),
		ClinicKind:          env("CLINIC_KIND", "GENERAL_MEDICAL"),
		ClinicAdminName:     os.Getenv("CLINIC_ADMIN_NAME"),
		ClinicAdminEmail:    first("CLINIC_ADMIN_EMAIL", "ADMIN_EMAIL"), // ADMIN_* are the old names
		ClinicAdminUsername: firstOr("admin", "CLINIC_ADMIN_USERNAME", "ADMIN_USERNAME"),
		ClinicAdminPassword: first("CLINIC_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
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

// first returns the first non-empty variable among keys.
func first(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func firstOr(def string, keys ...string) string {
	if v := first(keys...); v != "" {
		return v
	}
	return def
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
