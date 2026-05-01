package main

import (
	"errors"
	"flag"
	"flight-booking/internal/config"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	cfg := config.MustLoad()

	var migrationsPath string

	flag.StringVar(&migrationsPath, "migrations-path", "", "path to migrations")
	flag.Parse()

	if migrationsPath == "" {
		panic("migrations-path is required")
	}

	m, err := migrate.New("file://"+migrationsPath, fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", cfg.Storage.User, cfg.Storage.User, cfg.Storage.Address, cfg.Storage.Name))
	if err != nil {
		panic(err)
	}

	if err = m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("migrations not found")
			return
		}

		panic(err)
	}

	fmt.Println("migrations up")
}
