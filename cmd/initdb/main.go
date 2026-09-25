// Command initdb creates the database schema. Replaces the Python
// app/db/init_db.py entrypoint.
//
// TODO for myself: there is no .env.example committed, and .env is gitignored,
// so a fresh clone has no template for DATABASE_URL.
package main

import (
	"context"
	"log"

	"github.com/AhmedCSAhmed/conductor/internal/db"
)

func main() {
	ctx := context.Background()

	if err := db.Init(ctx); err != nil {
		log.Fatalf("initdb: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(ctx); err != nil {
		log.Fatalf("initdb: %v", err)
	}

	log.Println("schema is up to date")
}
