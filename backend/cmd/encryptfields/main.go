// Command encryptfields seals the clinical free-text columns that were saved in plain text before encryption at rest
// existed (bitácora, notas, recetas, antecedentes y alergias, detalles de cita). It is safe to run any number of
// times: values that are already sealed are skipped, and rows edited while it runs are left for the next run.
//
//	go run ./cmd/encryptfields --dry-run   # counts what would change, writes nothing
//	go run ./cmd/encryptfields             # seals everything in batches
//
// It uses the same JWT_SECRET / TOKEN_ENC_KEY as the server: with a different key nothing can be opened.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"github.com/madgrismad/miconsultorio/backend/internal/fieldcrypt"
)

func main() {
	dry := flag.Bool("dry-run", false, "only count; do not write")
	batch := flag.Int("batch", 500, "rows per batch")
	flag.Parse()
	if p := config.LoadDotEnv(); p != "" {
		log.Printf("Configuración cargada desde %s", p)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ring, err := fieldcrypt.NewRing(cfg.TokenEncKey)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	stats, err := fieldcrypt.EncryptAll(ctx, pool, ring, fieldcrypt.Options{DryRun: *dry, Batch: *batch})
	mode := "cifrado"
	if *dry {
		mode = "simulación (sin escribir)"
	}
	fmt.Printf("Modo: %s\n%-26s %8s %8s %8s %8s %8s %8s\n", mode, "columna", "con dato", "cifrados", "re-llave", "ya había", "ilegible", "cambió")
	failed := 0
	for _, s := range stats {
		fmt.Printf("%-26s %8d %8d %8d %8d %8d %8d\n", s.Table+"."+s.Column, s.Scanned, s.Sealed, s.Rekeyed, s.Already, s.Undecrypt, s.Raced)
		failed += s.Undecrypt
	}
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "Aviso: %d valores ya cifrados no se pueden abrir con la llave actual; no se tocaron (revisa docs/RESPALDOS.md, «Llave de cifrado»).\n", failed)
		os.Exit(3)
	}
}
