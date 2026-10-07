package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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
	startUncollectedBookingCleanup(queries)
	if err := handlers.InitAuth(context.Background()); err != nil {
		log.Fatal("Konnte OIDC nicht initialisieren:", err)
	}
	if err := handlers.EnsureDefaultAdmin(context.Background()); err != nil {
		log.Fatal("Konnte den Default-Admin nicht setzen:", err)
	}
	if err := handlers.InitMail(); err != nil {
		log.Fatal("Konnte E-Mail-Versand nicht initialisieren:", err)
	}

	r := gin.Default()
	trustedProxies := make([]string, 0)
	for _, proxy := range strings.Split(os.Getenv("GIN_TRUSTED_PROXIES"), ",") {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			trustedProxies = append(trustedProxies, proxy)
		}
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		log.Fatal("GIN_TRUSTED_PROXIES enthält eine ungültige IP-Adresse oder ein ungültiges CIDR-Netz:", err)
	}
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
	r.Use(sessions.Sessions("app_session", store))

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")

	// Lade alle Routen aus routes.go
	SetupRoutes(r)

	log.Println("Leih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}

func startUncollectedBookingCleanup(queries *db.Queries) {
	cleanup := func() {
		now := time.Now().UTC()
		pickupDayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		cancelled, err := queries.CancelUncollectedBookings(context.Background(), pickupDayStart)
		if err != nil {
			log.Printf("could not cancel uncollected bookings: %v", err)
			return
		}
		if cancelled > 0 {
			log.Printf("cancelled %d uncollected booking(s)", cancelled)
		}
	}

	cleanup()
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
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
