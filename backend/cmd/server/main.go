package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

// healthcheck asks the local server for /api/health (the image has no curl): exit 0 when it answers 200.
func healthcheck() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := http.Client{Timeout: 4 * time.Second}
	res, err := client.Get("http://" + addr + "/api/health")
	if err != nil || res.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}
	if p := config.LoadDotEnv(); p != "" {
		log.Printf("Configuración cargada desde %s", p)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	if err := api.SeedGrowthDefaults(ctx, pool); err != nil {
		log.Fatalf("growth tables: %v", err)
	}

	if cfg.PlatformEmail != "" && cfg.PlatformPassword != "" {
		created, err := db.EnsureFirstPlatformAdmin(ctx, pool, db.UserParams{
			Name: cfg.PlatformName, Email: cfg.PlatformEmail, Username: cfg.PlatformUsername, Password: cfg.PlatformPassword,
		})
		if err != nil {
			log.Fatalf("first-run platform admin: %v", err)
		}
		if created {
			log.Printf("Created platform administrator: sign in with %s or %s", cfg.PlatformEmail, cfg.PlatformUsername)
		}
	}
	if cfg.ClinicAdminEmail != "" && cfg.ClinicAdminPassword != "" {
		created, err := db.EnsureFirstClinic(ctx, pool, db.ClinicParams{
			Name: cfg.ClinicName, Kind: cfg.ClinicKind, Plan: "crecimiento", Status: "active", SetupDone: true,
			AdminName: cfg.ClinicAdminName, AdminEmail: cfg.ClinicAdminEmail, AdminUsername: cfg.ClinicAdminUsername, AdminPassword: cfg.ClinicAdminPassword,
		})
		if err != nil {
			// Not fatal: the server must still start (the platform panel and /register work without it).
			log.Printf("WARNING: sample clinic not created (%v). Usually the clinic admin's username/e-mail is already taken, "+
				"e.g. CLINIC_ADMIN_USERNAME equals PLATFORM_ADMIN_USERNAME. Usernames and e-mails are unique across the whole platform; "+
				"change CLINIC_ADMIN_USERNAME / CLINIC_ADMIN_EMAIL in .env or remove them.", err)
		} else if created {
			log.Printf("Created clinic %q: sign in with %s or %s", cfg.ClinicName, cfg.ClinicAdminEmail, cfg.ClinicAdminUsername)
		}
	}

	handler, startBackground := api.NewApp(pool, cfg)
	startBackground(ctx)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if cfg.StaticDir != "" {
		log.Printf("Sirviendo la interfaz desde %s", cfg.StaticDir)
	} else {
		log.Printf("Aviso: no se encontró frontend/build; solo estará disponible la API (/api).")
	}
	log.Printf("Caresia lista en http://localhost%s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
