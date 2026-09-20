package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"html/template"
	"io"
	"leiheV2/db" // Dein generierter Code!
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var bggMutex sync.Mutex
var bggLastCall time.Time

// var oauth2Config oauth2.Config
// var verifier *oidc.IDTokenVerifier
var queries *db.Queries

func enforceBGGCooldown() {
	bggMutex.Lock()
	defer bggMutex.Unlock()

	cooldown := 2 * time.Second
	elapsed := time.Since(bggLastCall)
	if elapsed < cooldown {
		time.Sleep(cooldown - elapsed)
	}
	bggLastCall = time.Now()
}

func handleDevLogin(c *gin.Context) {
	role := c.Query("role")
	if role != "admin" && role != "maintainer" {
		role = "user" // Fallback
	}

	// Dummy-Nutzer in der Datenbank anlegen oder updaten (mit deiner neuen sqlc Query!)
	dummySub := "dev-dummy-" + role
	_, err := queries.UpsertUser(context.Background(), db.UpsertUserParams{
		OidcSub: dummySub,
		Email:   role + "@fius.local",
		Name:    "Test " + strings.ToUpper(role),
		Role:    db.UserRole(role), // Setzt die Rolle auf user, maintainer oder admin
	})

	if err != nil {
		c.String(http.StatusInternalServerError, "DB Fehler beim Dev-Login: "+err.Error())
		return
	}

	// Session setzen, genau wie es der echte OIDC-Callback tun würde
	session := sessions.Default(c)
	session.Set("user_id", dummySub)
	session.Set("user_role", role)
	session.Save()

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

// RequireRole prüft, ob der Nutzer eingeloggt ist UND eine der erlaubten Rollen hat
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		// 1. Gar nicht eingeloggt?
		if userRole == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bitte einloggen"})
			return
		}

		roleStr := userRole.(string)

		// 2. Darf die Rolle auf diese Route zugreifen?
		hasAccess := false
		for _, allowed := range allowedRoles {
			if roleStr == allowed {
				hasAccess = true
				break
			}
		}

		// Admin-Override: Admins dürfen immer alles, was Maintainer dürfen
		if roleStr == "admin" {
			hasAccess = true
		}

		if !hasAccess {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Keine Berechtigung"})
			return
		}

		// Alles cool, lass den Nutzer durch!
		c.Next()
	}
}

type BGGSearchResponse struct {
	Items []struct {
		ID   string `xml:"id,attr"`
		Name struct {
			Value string `xml:"value,attr"`
		} `xml:"name"`
		Year struct {
			Value string `xml:"value,attr"`
		} `xml:"yearpublished"`
	} `xml:"item"`
}

type BGGThingResponse struct {
	Items []struct {
		ID         string `xml:"id,attr"`
		Image      string `xml:"image"`
		MinPlayers struct {
			Value string `xml:"value,attr"`
		} `xml:"minplayers"`
		MaxPlayers struct {
			Value string `xml:"value,attr"`
		} `xml:"maxplayers"`
		MinPlaytime struct {
			Value string `xml:"value,attr"`
		} `xml:"minplaytime"`
		MaxPlaytime struct {
			Value string `xml:"value,attr"`
		} `xml:"maxplaytime"`
		Links []struct {
			Type  string `xml:"type,attr"`
			Value string `xml:"value,attr"`
		} `xml:"link"`
	} `xml:"item"`
}

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
	queries = db.New(database)

	// 3. Gin Webserver einrichten
	r := gin.Default()

	// Session-Middleware einhängen
	store := cookie.NewStore([]byte("geheimes-fius-session-secret"))
	r.Use(sessions.Sessions("fius_session", store))

	// Dev-Login Route registrieren
	r.GET("/dev-login", handleDevLogin)

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
		c.Redirect(http.StatusFound, "/admin")
	})

	// Startseite
	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		items, err := queries.ListItems(context.Background())
		if err != nil {
			c.String(http.StatusInternalServerError, "Datenbankfehler")
			return
		}

		// Alle Spielnamen in eine einfache Liste extrahieren
		names := make([]string, 0, len(items))
		for _, item := range items {
			names = append(names, item.Name)
		}

		// Als JSON verpacken, damit JavaScript (Alpine) es nativ lesen kann
		namesJSON, _ := json.Marshal(names)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"TotalItems": len(items),
			"GameNames":  template.JS(namesJSON),
			"UserRole":   userRole, // <-- Hier hinzugefügt
		})
	})

	r.GET("/games", func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		items, err := queries.ListItemsWithCategories(context.Background())
		if err != nil {
			c.String(http.StatusInternalServerError, "Datenbankfehler")
			return
		}

		categories, _ := queries.ListCategories(context.Background())

		c.HTML(http.StatusOK, "games.html", gin.H{
			"Games":      items,
			"Categories": categories,
			"UserRole":   userRole,
		})
	})

	// Admin: Spieleverwaltung anzeigen
	r.GET("/admin", func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		items, _ := queries.ListItems(context.Background())
		// ... Error handling ...
		c.HTML(http.StatusOK, "managment.html", gin.H{
			"Games":    items,
			"UserRole": userRole,
		})
	})

	// GET: Zeigt das nächste Bild an
	r.GET("/setup-bilder", func(c *gin.Context) {
		skipParam := c.Query("skip")

		// Übersprungene Bilder in eine Map packen
		skippedMap := make(map[string]bool)
		for _, s := range strings.Split(skipParam, ",") {
			if s != "" {
				skippedMap[s] = true
			}
		}

		files, err := os.ReadDir("./static/custom_images")
		if err != nil || len(files) == 0 {
			c.String(http.StatusOK, "Keine Bilder gefunden.")
			return
		}

		games, _ := queries.ListItems(c)

		mappedImages := make(map[string]bool)
		for _, game := range games {
			if game.CustomImageUrl.Valid {
				fileName := filepath.Base(game.CustomImageUrl.String)
				mappedImages[fileName] = true
			}
		}

		var currentImage string
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			// Überspringen, wenn in DB oder in der Skip-Liste
			if mappedImages[file.Name()] || skippedMap[file.Name()] {
				continue
			}
			currentImage = file.Name()
			break
		}

		if currentImage == "" {
			c.HTML(http.StatusOK, "setup-bilder.html", gin.H{
				"Finished": true,
			})
			return
		}

		c.HTML(http.StatusOK, "setup-bilder.html", gin.H{
			"CurrentImage": currentImage,
			"CurrentSkips": skipParam, // Wir geben die bisherige Liste ans Template weiter
			"Games":        games,
		})
	})

	// POST: Speichert die Zuordnung und leitet weiter
	r.POST("/setup-bilder", func(c *gin.Context) {
		imageName := c.PostForm("image_name")
		itemIDStr := c.PostForm("item_id")
		skips := c.PostForm("skips") // Retten der Skip-Liste beim Speichern

		itemID, err := strconv.Atoi(itemIDStr)
		if err == nil {
			dbErr := queries.UpdateItemCustomImage(c, db.UpdateItemCustomImageParams{
				CustomImageUrl: sql.NullString{String: "/static/custom_images/" + imageName, Valid: true},
				ID:             int32(itemID),
			})
			if dbErr != nil {
				log.Println("FEHLER BEIM BILD-SPEICHERN:", dbErr)
			}
		}

		// Leite um und hänge die Skip-Liste wieder an, falls vorhanden
		redirectURL := "/setup-bilder"
		if skips != "" {
			redirectURL += "?skip=" + skips
		}
		c.Redirect(http.StatusFound, redirectURL)
	})

	// Bestehendes Spiel aktualisieren
	r.POST("/api/items/update", func(c *gin.Context) {
		// Wir können die gleiche Struktur wie beim Erstellen nutzen, plus die ID
		type UpdateItemRequest struct {
			ID          int32  `form:"id" binding:"required"`
			Name        string `form:"name" binding:"required"`
			MinPlayers  int32  `form:"min_players"`
			MaxPlayers  int32  `form:"max_players"`
			MinPlaytime int32  `form:"min_playtime"`
			MaxPlaytime int32  `form:"max_playtime"`
		}

		var req UpdateItemRequest
		if err := c.ShouldBind(&req); err != nil {
			c.String(http.StatusBadRequest, "Ungültige Eingabe: "+err.Error())
			return
		}

		err := queries.UpdateItem(context.Background(), db.UpdateItemParams{
			ID:          req.ID,
			Name:        req.Name,
			MinPlayers:  req.MinPlayers,
			MaxPlayers:  req.MaxPlayers,
			MinPlaytime: req.MinPlaytime,
			MaxPlaytime: req.MaxPlaytime,
		})

		if err != nil {
			log.Println("FEHLER BEIM UPDATEN:", err)
			c.String(http.StatusInternalServerError, "Konnte Änderungen nicht speichern")
			return
		}

		c.Redirect(http.StatusFound, "/admin")
	})

	// --- BGG XML STRUKTUREN ---
	type BGGSearchResponse struct {
		Items []struct {
			ID   string `xml:"id,attr"`
			Name struct {
				Value string `xml:"value,attr"`
			} `xml:"name"`
			Year struct {
				Value string `xml:"value,attr"`
			} `xml:"yearpublished"`
		} `xml:"item"`
	}

	type BGGThingResponse struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Image      string `xml:"image"`
			MinPlayers struct {
				Value string `xml:"value,attr"`
			} `xml:"minplayers"`
			MaxPlayers struct {
				Value string `xml:"value,attr"`
			} `xml:"maxplayers"`
			MinPlaytime struct {
				Value string `xml:"value,attr"`
			} `xml:"minplaytime"`
			MaxPlaytime struct {
				Value string `xml:"value,attr"`
			} `xml:"maxplaytime"`
			Links []struct {
				Type  string `xml:"type,attr"`
				Value string `xml:"value,attr"`
			} `xml:"link"`
		} `xml:"item"`
	}

	// --- BGG API ROUTEN ---

	// 1. Suche: Gibt eine Liste von möglichen Spielen zurück (Fuzzy)
	// 1. Suche: Gibt eine Liste von möglichen Spielen zurück (Fuzzy)
	r.GET("/api/bgg/search", func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Leerer Suchbegriff"})
			return
		}

		bggURL := "https://boardgamegeek.com/xmlapi2/search?type=boardgame&query=" + url.QueryEscape(query)
		enforceBGGCooldown()

		// Eigener Client mit FIUS User-Agent
		req, _ := http.NewRequest("GET", bggURL, nil)
		req.Header.Set("User-Agent", "FIUS-LeiheV2/1.0 (University of Stuttgart)")
		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG API nicht erreichbar"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// Fehler gnadenlos ins Terminal loggen
		if resp.StatusCode != http.StatusOK {
			log.Printf("BGG Search Fehler: Status %d - %s\n", resp.StatusCode, string(body))
			c.JSON(resp.StatusCode, gin.H{"error": "BGG hat die Anfrage abgelehnt"})
			return
		}

		var bggResult BGGSearchResponse
		if err := xml.Unmarshal(body, &bggResult); err != nil {
			log.Println("XML Parse Fehler (Search):", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG XML konnte nicht gelesen werden"})
			return
		}

		c.JSON(http.StatusOK, bggResult.Items)
	})

	// 2. Details: Holt die genauen Daten für eine spezifische BGG-ID
	r.GET("/api/bgg/details", func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Keine ID übergeben"})
			return
		}

		bggURL := "https://boardgamegeek.com/xmlapi2/thing?id=" + id
		enforceBGGCooldown()

		req, _ := http.NewRequest("GET", bggURL, nil)
		req.Header.Set("User-Agent", "FIUS-LeiheV2/1.0 (University of Stuttgart)")
		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG API nicht erreichbar"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			log.Printf("BGG Details Fehler: Status %d - %s\n", resp.StatusCode, string(body))
			c.JSON(resp.StatusCode, gin.H{"error": "BGG hat die Anfrage abgelehnt"})
			return
		}

		var bggResult BGGThingResponse
		if err := xml.Unmarshal(body, &bggResult); err != nil {
			log.Println("XML Parse Fehler (Details):", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG XML konnte nicht gelesen werden"})
			return
		}

		if len(bggResult.Items) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Spiel nicht gefunden"})
			return
		}

		item := bggResult.Items[0]
		var categories []string
		for _, link := range item.Links {
			if link.Type == "boardgamecategory" {
				categories = append(categories, link.Value)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"bgg_id":       item.ID,
			"image_url":    item.Image,
			"min_players":  item.MinPlayers.Value,
			"max_players":  item.MaxPlayers.Value,
			"min_playtime": item.MinPlaytime.Value,
			"max_playtime": item.MaxPlaytime.Value,
			"categories":   categories,
		})
	})

	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	// 5. Server starten
	log.Println("🚀 FIUS Verleih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}
