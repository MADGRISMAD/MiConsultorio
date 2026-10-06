// Command createclinic provisions a clinic and its first administrator.
//
//	go run ./cmd/createclinic -name "Clínica Sol" -email dra@clinica.mx -username dra
//
// The administrator's password is read from CARESIA_ADMIN_PASSWORD, or prompted on stdin.
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
	email := flag.String("email", "", "administrator's e-mail, used to sign in (required)")
	adminName := flag.String("admin-name", "", "administrator's full name (defaults to the username)")
	phone := flag.String("phone", "", "clinic phone number")
	address := flag.String("address", "", "clinic address")
	kind := flag.String("kind", "GENERAL_MEDICAL", "GENERAL_MEDICAL, DENTAL, PEDIATRICS, INTERNAL_MEDICINE, PHYSIOTHERAPY, NUTRITION, PSYCHOLOGY, DERMATOLOGY, GYNECOLOGY, ORTHOPEDICS, VETERINARY or CHIROPRACTIC")
	plan := flag.String("plan", "consultorio", "consultorio, clinica or empresarial")
	status := flag.String("status", "active", "active, or trialing for a 14-day trial")
	username := flag.String("username", "admin", "administrator's username")
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
		Name: *name, Kind: *kind, Plan: *plan, Status: *status, SetupDone: true, Phone: *phone, Address: *address,
		AdminName: *adminName, AdminEmail: *email, AdminUsername: *username, AdminPassword: password,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Clinic %q created (id %s). Sign in with %s or %s.\n", *name, clinicID, strings.ToLower(*email), *username)
}
