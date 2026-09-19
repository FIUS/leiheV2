package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"

	"leiheV2/db" // Dein generierter Code!

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // Postgres Treiber laden
)

func main() {
	// 1. Datenbankverbindung herstellen
	connStr := os.Getenv("DB_URL")
	if connStr == "" {
		// Bequemer Fallback für deine lokale Windows-Entwicklung
		connStr = "host=localhost port=5432 user=fius password=pleasechangeme dbname=verleih sslmode=disable"
	}

	database, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Konnte nicht zur Datenbank verbinden:", err)
	}
	defer database.Close()

	// 2. sqlc Queries initialisieren
	queries := db.New(database)

	// 3. Gin Webserver einrichten
	r := gin.Default()

	// 4. Erste API-Route definieren
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")

	// Eigene Struktur für eingehende Daten (unterstützt JSON und HTML-Formulare)
	type CreateItemRequest struct {
		Name        string `form:"name" json:"name" binding:"required"`
		MinPlayers  int32  `form:"min_players" json:"min_players"`
		MaxPlayers  int32  `form:"max_players" json:"max_players"`
		MinPlaytime int32  `form:"min_playtime" json:"min_playtime"`
		MaxPlaytime int32  `form:"max_playtime" json:"max_playtime"`
		BggID       string `form:"bgg_id" json:"bgg_id"`
		ImageUrl    string `form:"image_url" json:"image_url"`
	}

	// Neues Item anlegen (POST-Route flexibel für JSON & Formular)
	r.POST("/api/items", func(c *gin.Context) {
		var req CreateItemRequest

		// ShouldBind erkennt anhand des Headers automatisch, ob es JSON oder ein Formular ist
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe: " + err.Error()})
			return
		}

		// Daten an die von sqlc generierte Struktur übergeben
		dbReq := db.CreateItemParams{
			Name:        req.Name,
			MinPlayers:  req.MinPlayers,
			MaxPlayers:  req.MaxPlayers,
			MinPlaytime: req.MinPlaytime,
			MaxPlaytime: req.MaxPlaytime,
			Status:      "available", // Standardstatus für neue Spiele
		}

		// Falls du die Bild-Felder im Schema schon angepasst hast:
		// dbReq.ImageUrl = sql.NullString{String: req.ImageUrl, Valid: req.ImageUrl != ""}

		_, err := queries.CreateItem(context.Background(), dbReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Konnte Item nicht in DB speichern"})
			return
		}

		// Nach erfolgreichem Speichern direkt zurück zur Ansicht leiten
		c.Redirect(http.StatusFound, "/admin/games")
	})

	// Startseite
	r.GET("/", func(c *gin.Context) {
		items, err := queries.ListItems(context.Background())
		if err != nil {
			c.String(http.StatusInternalServerError, "Datenbankfehler")
			return
		}

		// Alle Spielnamen in eine einfache Liste extrahieren
		var names []string
		for _, item := range items {
			names = append(names, item.Name)
		}

		// Als JSON verpacken, damit JavaScript (Alpine) es nativ lesen kann
		namesJSON, _ := json.Marshal(names)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"TotalItems": len(items),
			"GameNames":  template.JS(namesJSON), // template.JS sichert sauberen JS-Code
		})
	})

	r.GET("/games", func(c *gin.Context) {
		items, _ := queries.ListItems(context.Background())
		// ... Error handling ...
		c.HTML(http.StatusOK, "games.html", gin.H{
			"Games": items,
		})
	})
	// Admin: Spieleverwaltung anzeigen (angepasste Route)
	r.GET("/admin/games", func(c *gin.Context) {
		items, _ := queries.ListItems(context.Background())
		// ... Error handling ...
		c.HTML(http.StatusOK, "managment.html", gin.H{
			"Games": items,
		})
	})

	// 5. Server starten
	log.Println("🚀 FIUS Verleih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}
