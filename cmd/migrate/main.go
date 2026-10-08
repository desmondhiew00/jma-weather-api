// Command migrate applies or rolls back schema migrations by hand.
//
// The worker migrates on startup, so this exists for the times you want to
// migrate without starting a worker, most usefully `down` during development.
package main

import (
	"fmt"
	"os"

	"github.com/desmondhiew00/jma-weather-api/internal/config"
	"github.com/desmondhiew00/jma-weather-api/internal/migrate"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: migrate up|down")
	}

	cfg, err := config.Load()
	if err != nil {
		fail(err.Error())
	}

	switch os.Args[1] {
	case "up":
		if err := migrate.Up(cfg.DatabaseURL); err != nil {
			fail(err.Error())
		}

		fmt.Println("migrations applied")
	case "down":
		if err := migrate.Down(cfg.DatabaseURL); err != nil {
			fail(err.Error())
		}

		fmt.Println("rolled back one migration")
	default:
		fail("usage: migrate up|down")
	}
}

func fail(msg string) {
	_, _ = os.Stderr.WriteString(msg + "\n")

	os.Exit(1)
}
