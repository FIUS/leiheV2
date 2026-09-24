package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"leiheV2/db"
	"leiheV2/handlers"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	connStr := os.Getenv("DB_URL")
	if connStr == "" {
		log.Fatal("DB_URL muss gesetzt sein")
	}

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Konnte nicht zur Datenbank verbinden:", err)
	}
	defer database.Close()

	// Initialisiere die generierten sqlc Queries und übergebe sie an die Handler
	queries := db.New(database)
	handlers.InitQueries(queries)
	if err := handlers.InitAuth(context.Background()); err != nil {
		log.Fatal("Konnte OIDC nicht initialisieren:", err)
	}
	if err := handlers.PromoteInitialAdmin(context.Background()); err != nil {
		log.Fatal("Konnte den Initial-Admin nicht setzen:", err)
	}
	if err := handlers.InitMail(); err != nil {
		log.Fatal("Konnte E-Mail-Versand nicht initialisieren:", err)
	}

	r := gin.Default()
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET muss gesetzt sein")
	}
	sessionCookieSecure, err := requiredBoolEnv("SESSION_COOKIE_SECURE")
	if err != nil {
		log.Fatal(err)
	}
	store := cookie.NewStore([]byte(sessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		Secure:   sessionCookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(sessions.Sessions("fius_session", store))

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")

	// Lade alle Routen aus routes.go
	SetupRoutes(r)

	log.Println("🚀 FIUS Verleih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}

func requiredBoolEnv(key string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return false, fmt.Errorf("%s muss gesetzt sein", key)
	}
	switch value {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s muss true oder false sein", key)
	}
}
