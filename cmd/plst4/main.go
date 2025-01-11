package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/btmxh/plst4/internal/auth"
	"github.com/btmxh/plst4/internal/db"
	"github.com/btmxh/plst4/internal/mailer"
	"github.com/btmxh/plst4/internal/routes"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Unable to load .env file: %w", err)
		os.Exit(1)
	}

	logLevel := slog.LevelDebug
	if levelStr, ok := os.LookupEnv("LOG_LEVEL"); ok {
		if err = logLevel.UnmarshalText([]byte(levelStr)); err != nil {
			fmt.Println("(warn) Invalid value for LOG_LEVEL environment variable")
		}
	}

	logHandler := tint.NewHandler(os.Stderr, &tint.Options{
		Level: logLevel,
	})

	slog.SetDefault(slog.New(logHandler))

	dbUrl, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		panic("Required environment veriable DATABASE_URL not set")
	}

	err = db.InitDB(dbUrl)
	if err != nil {
		panic(err)
	}
	defer db.CloseDB()
	slog.Info("Database connection initialized")

	if err = mailer.InitMailer(); err != nil {
		panic(err)
	}

	if err = auth.InitJWT(); err != nil {
		panic(err)
	}

	addr, ok := os.LookupEnv("PLST4_ADDR")
	if !ok {
		addr = "localhost:6972"
		slog.Info("PLST4_ADDR not provided, using default '" + addr + "'")
	}

	cert, hasCert := os.LookupEnv("HTTPS_CERT_FILE")
	key, hasKey := os.LookupEnv("HTTPS_KEY_FILE")

	router := routes.CreateMainRouter()

	if hasKey && hasCert {
		slog.Info("Starting HTTPS server", slog.String("addr", addr), slog.String("cert", cert), slog.String("key", key))
		err = http.ListenAndServeTLS(addr, cert, key, router)
	} else {
		slog.Info("Starting HTTP server", slog.String("addr", addr))
		err = http.ListenAndServe(addr, router)
	}

	if err != nil {
		panic(err)
	}
}
