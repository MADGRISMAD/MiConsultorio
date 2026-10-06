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

	"github.com/madgrismad/miconsultorio/backend/internal/config"
	"github.com/madgrismad/miconsultorio/backend/internal/db"
)

func main() {
	name := flag.String("name", "", "clinic name (required)")
	email := flag.String("email", "", "clinic email, used to sign in (required)")
	phone := flag.String("phone", "", "clinic phone number")
	address := flag.String("address", "", "clinic address")
	image := flag.String("image", "", "clinic image URL")
	username := flag.String("username", "admin", "first administrator's username")
	flag.Parse()
	config.LoadDotEnv()

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
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}
	clinicID, err := db.CreateClinic(ctx, pool, db.ClinicParams{
		Name: *name, Email: *email, Phone: *phone, Address: *address, ImageURL: *image, Username: *username, Password: password,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Clinic %q created (id %s). Sign in with email %s and user %s.\n", *name, clinicID, strings.ToLower(*email), *username)
}
