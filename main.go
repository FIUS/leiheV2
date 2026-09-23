package main

import (
	"context"
	"database/sql"
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
		connStr = "host=localhost port=5432 user=fius password=pleasechangeme dbname=verleih sslmode=disable"
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

	r := gin.Default()
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET muss gesetzt sein")
	}
	store := cookie.NewStore([]byte(sessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		Secure:   envBool("SESSION_COOKIE_SECURE", false),
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

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}
