package main

import (
	"database/sql"
	"log"
	"os"

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

	r := gin.Default()
	store := cookie.NewStore([]byte("geheimes-fius-session-secret"))
	r.Use(sessions.Sessions("fius_session", store))

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")

	// Lade alle Routen aus routes.go
	SetupRoutes(r)

	log.Println("🚀 FIUS Verleih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}
