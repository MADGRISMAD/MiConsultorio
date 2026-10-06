package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/madgrismad/miconsultorio/backend/internal/api"
	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

func main() {
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

	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		created, err := db.EnsureFirstClinic(ctx, pool, db.ClinicParams{
			Name: cfg.ClinicName, Email: cfg.AdminEmail, Username: cfg.AdminUsername, Password: cfg.AdminPassword,
		})
		if err != nil {
			log.Fatalf("first-run setup: %v", err)
		}
		if created {
			log.Printf("Created clinic %q: sign in with email %s and user %s", cfg.ClinicName, cfg.AdminEmail, cfg.AdminUsername)
		}
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewRouter(pool, cfg),
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
