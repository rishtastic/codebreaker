package main

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"time"

	"rishabhdaga.com/codebreaker/internal/server"
)

func main() {
	password := os.Getenv("CODEBREAKER_PASSWORD")
	if password == "" {
		log.Fatal("CODEBREAKER_PASSWORD must be set")
	}

	sessionKey := os.Getenv("CODEBREAKER_SESSION_KEY")
	if sessionKey == "" {
		if os.Getenv("CODEBREAKER_ENV") == "production" {
			log.Fatal("CODEBREAKER_SESSION_KEY must be set in production")
		}
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			log.Fatalf("generate development session key: %v", err)
		}
		sessionKey = base64.RawURLEncoding.EncodeToString(buf)
		log.Print("using an ephemeral development session key")
	}
	if len(sessionKey) < 32 {
		log.Fatal("CODEBREAKER_SESSION_KEY must contain at least 32 characters")
	}

	app := server.New(server.Config{
		Password:      password,
		SessionKey:    []byte(sessionKey),
		SecureCookies: os.Getenv("CODEBREAKER_ENV") == "production",
		Now:           time.Now,
	})
	defer app.Close()

	httpServer := &http.Server{
		Addr:              "127.0.0.1:8081",
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       75 * time.Second,
	}
	log.Printf("listening on %s", httpServer.Addr)
	log.Fatal(httpServer.ListenAndServe())
}
