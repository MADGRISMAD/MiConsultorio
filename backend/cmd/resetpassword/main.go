// Command resetpassword sets a new password for any account, by e-mail or username.
// Useful when the only administrator is locked out.
//
//	go run ./cmd/resetpassword -user admin@caresia.com
//
// The new password is read from CARESIA_NEW_PASSWORD, or prompted on stdin.
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
	user := flag.String("user", "", "e-mail or username of the account (required)")
	flag.Parse()
	config.LoadDotEnv()

	if *user == "" {
		flag.Usage()
		os.Exit(2)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	password := os.Getenv("CARESIA_NEW_PASSWORD")
	if password == "" {
		fmt.Fprint(os.Stderr, "New password: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		password = strings.TrimRight(line, "\r\n")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	name, err := db.ResetPassword(ctx, pool, *user, password)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Password updated for %s. Their open sessions were closed.\n", name)
}
