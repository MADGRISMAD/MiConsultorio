// Command createclinic provisions a clinic and its first administrator.
//
//	go run ./cmd/createclinic -name "Clínica Sol" -email contacto@clinica.mx -username admin
//
// The admin password is read from CARESIA_ADMIN_PASSWORD, or prompted on stdin.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/madgrismad/miconsultorio/backend/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	name := flag.String("name", "", "clinic name (required)")
	email := flag.String("email", "", "clinic email, used to sign in (required)")
	phone := flag.String("phone", "", "clinic phone number")
	address := flag.String("address", "", "clinic address")
	image := flag.String("image", "", "clinic image URL")
	username := flag.String("username", "admin", "first administrator's username")
	flag.Parse()

	if *name == "" || *email == "" {
		flag.Usage()
		os.Exit(2)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	password := os.Getenv("CARESIA_ADMIN_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "Admin password: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		password = strings.TrimRight(line, "\r\n")
	}
	if utf8.RuneCountInString(password) < 8 || len(password) > 72 {
		log.Fatal("password must be 8 to 72 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var clinicID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO clinics (name, email, phone_number, address, image_url) VALUES ($1, lower($2), $3, $4, $5) RETURNING id`,
		*name, *email, *phone, *address, *image).Scan(&clinicID); err != nil {
		log.Fatalf("create clinic: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO users (clinic_id, username, password_hash, permissions) VALUES ($1, $2, $3, $4)`,
		clinicID, *username, string(hash),
		[]string{"adminUsers", "adminAppointments", "adminHistorials", "navHistorials", "navAppointments"}); err != nil {
		log.Fatalf("create admin: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Clinic %q created (id %s). Sign in with email %s and user %s.\n", *name, clinicID, strings.ToLower(*email), *username)
}
