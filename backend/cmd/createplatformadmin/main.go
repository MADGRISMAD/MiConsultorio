// Command createplatformadmin creates an account for the people who run Caresia itself
// (outside any clinic): platform_admin (full control) or platform_support (read-only).
//
//	go run ./cmd/createplatformadmin -name "Ana" -email ana@caresia.com -username ana
//
// The password is read from CARESIA_ADMIN_PASSWORD, or prompted on stdin.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

func main() {
	name := flag.String("name", "", "full name (required)")
	email := flag.String("email", "", "e-mail, used to sign in (required)")
	username := flag.String("username", "", "username (required)")
	role := flag.String("role", "platform_admin", "platform_admin or platform_support")
	flag.Parse()
	config.LoadDotEnv()

	if *name == "" || *email == "" || *username == "" {
		flag.Usage()
		os.Exit(2)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	password := os.Getenv("CARESIA_ADMIN_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "Password: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		password = strings.TrimRight(line, "\r\n")
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
	id, err := db.CreatePlatformUser(ctx, pool, db.UserParams{Name: *name, Email: *email, Username: *username, Password: password, Role: *role})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Platform account created (id %s, role %s). Sign in with %s or %s.\n", id, *role, strings.ToLower(*email), *username)
}
