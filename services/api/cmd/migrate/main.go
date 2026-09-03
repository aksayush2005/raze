// Command migrate applies database migrations via goose.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

func main() {
	dir := flag.String("dir", "../../database/migrations", "migrations directory")
	down := flag.Bool("down", false, "roll back the latest migration instead of applying up")
	flag.Parse()

	_ = godotenv.Load() // best-effort; already-set env vars take precedence

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required (see .env.example)")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	if *down {
		if err := goose.Down(db, *dir); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("rolled back one migration")
		return
	}
	if err := goose.Up(db, *dir); err != nil {
		log.Fatalf("migrate up: %v", err)
	}
	fmt.Println("migrations applied")
}
