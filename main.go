package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"leiheV2/db" // Dein generierter Code!

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// ==========================================
// GLOBALE VARIABLEN & MIDDLEWARES
// ==========================================

var bggMutex sync.Mutex
var bggLastCall time.Time
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

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		if userRole == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bitte einloggen"})
			return
		}

		roleStr := userRole.(string)
		hasAccess := false
		for _, allowed := range allowedRoles {
			if roleStr == allowed {
				hasAccess = true
				break
			}
		}

		if roleStr == "admin" {
			hasAccess = true
		}

		if !hasAccess {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Keine Berechtigung"})
			return
		}
		c.Next()
	}
}

// ==========================================
// DATENSTRUKTUREN (REQUESTS & RESPONSES)
// ==========================================

type CreateItemRequest struct {
	Name        string `form:"name" binding:"required"`
	MinPlayers  int32  `form:"min_players"`
	MaxPlayers  int32  `form:"max_players"`
	MinPlaytime int32  `form:"min_playtime"`
	MaxPlaytime int32  `form:"max_playtime"`
	Room        string `form:"room"`
	Shelf       string `form:"shelf"`
}

type UpdateItemRequest struct {
	ID          int32  `form:"id" binding:"required"`
	Name        string `form:"name" binding:"required"`
	MinPlayers  int32  `form:"min_players"`
	MaxPlayers  int32  `form:"max_players"`
	MinPlaytime int32  `form:"min_playtime"`
	MaxPlaytime int32  `form:"max_playtime"`
	Room        string `form:"room"`
	Shelf       string `form:"shelf"`
}

type CreateUserRequest struct {
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name" binding:"required"`
	Email     string `form:"email" binding:"required"`
	Role      string `form:"role"`
	OidcSub   string `form:"oidc_sub"`
}

type UpdateUserRequest struct {
	ID        int32  `form:"id" binding:"required"`
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name" binding:"required"`
	Email     string `form:"email" binding:"required"`
	Role      string `form:"role" binding:"required"`
	OidcSub   string `form:"oidc_sub"`
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

// ==========================================
// MAIN & SETUP
// ==========================================

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

	queries = db.New(database)

	r := gin.Default()
	store := cookie.NewStore([]byte("geheimes-fius-session-secret"))
	r.Use(sessions.Sessions("fius_session", store))

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")

	// ==========================================
	// AUTHENTIFIZIERUNG
	// ==========================================
	r.GET("/dev-login", func(c *gin.Context) {
		role := c.Query("role")
		if role != "admin" && role != "maintainer" {
			role = "user"
		}

		dummySub := "dev-dummy-" + role
		_, err := queries.UpsertUser(context.Background(), db.UpsertUserParams{
			OidcSub:   dummySub,
			Email:     role + "@fius.local",
			FirstName: "Test",
			LastName:  strings.ToUpper(role),
			Role:      db.UserRole(role),
		})

		if err != nil {
			c.String(http.StatusInternalServerError, "DB Fehler beim Dev-Login: "+err.Error())
			return
		}

		session := sessions.Default(c)
		session.Set("user_id", dummySub)
		session.Set("user_role", role)
		session.Save()
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	// ==========================================
	// SEITEN (HTML RENDERING)
	// ==========================================
	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		items, err := queries.ListItems(context.Background())
		if err != nil {
			c.String(http.StatusInternalServerError, "Datenbankfehler")
			return
		}

		names := make([]string, 0, len(items))
		for _, item := range items {
			names = append(names, item.Name)
		}
		namesJSON, _ := json.Marshal(names)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"TotalItems": len(items),
			"GameNames":  template.JS(namesJSON),
			"UserRole":   session.Get("user_role"),
		})
	})

	r.GET("/games", func(c *gin.Context) {
		session := sessions.Default(c)
		items, err := queries.ListItemsWithCategories(context.Background())
		if err != nil {
			c.String(http.StatusInternalServerError, "Datenbankfehler")
			return
		}
		categories, _ := queries.ListCategories(context.Background())

		c.HTML(http.StatusOK, "games.html", gin.H{
			"Games":      items,
			"Categories": categories,
			"UserRole":   session.Get("user_role"),
		})
	})

	r.GET("/admin", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		session := sessions.Default(c)
		items, _ := queries.ListItems(context.Background())
		users, _ := queries.ListUsers(context.Background())

		c.HTML(http.StatusOK, "managment.html", gin.H{
			"Games":    items,
			"Users":    users,
			"UserRole": session.Get("user_role"),
		})
	})

	r.GET("/setup-bilder", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		skipParam := c.Query("skip")
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
				mappedImages[filepath.Base(game.CustomImageUrl.String)] = true
			}
		}

		var currentImage string
		for _, file := range files {
			if file.IsDir() || mappedImages[file.Name()] || skippedMap[file.Name()] {
				continue
			}
			currentImage = file.Name()
			break
		}

		if currentImage == "" {
			c.HTML(http.StatusOK, "setup-bilder.html", gin.H{"Finished": true})
			return
		}

		c.HTML(http.StatusOK, "setup-bilder.html", gin.H{
			"CurrentImage": currentImage,
			"CurrentSkips": skipParam,
			"Games":        games,
		})
	})

	r.POST("/setup-bilder", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		imageName := c.PostForm("image_name")
		itemIDStr := c.PostForm("item_id")
		skips := c.PostForm("skips")

		if itemID, err := strconv.Atoi(itemIDStr); err == nil {
			_ = queries.UpdateItemCustomImage(c, db.UpdateItemCustomImageParams{
				CustomImageUrl: sql.NullString{String: "/static/custom_images/" + imageName, Valid: true},
				ID:             int32(itemID),
			})
		}

		redirectURL := "/setup-bilder"
		if skips != "" {
			redirectURL += "?skip=" + skips
		}
		c.Redirect(http.StatusFound, redirectURL)
	})

	// ==========================================
	// API: ITEMS (SPIELE)
	// ==========================================
	r.POST("/api/items", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		var req CreateItemRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe: " + err.Error()})
			return
		}

		_, err := queries.CreateItem(context.Background(), db.CreateItemParams{
			Name:        req.Name,
			MinPlayers:  req.MinPlayers,
			MaxPlayers:  req.MaxPlayers,
			MinPlaytime: req.MinPlaytime,
			MaxPlaytime: req.MaxPlaytime,
			Status:      "available",
			Room:        sql.NullString{String: req.Room, Valid: req.Room != ""},
			Shelf:       sql.NullString{String: req.Shelf, Valid: req.Shelf != ""},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Konnte Item nicht speichern"})
			return
		}
		c.Redirect(http.StatusFound, "/admin?tab=spiele")
	})

	r.POST("/api/items/update", RequireRole("admin", "maintainer"), func(c *gin.Context) {
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
			Room:        sql.NullString{String: req.Room, Valid: req.Room != ""},
			Shelf:       sql.NullString{String: req.Shelf, Valid: req.Shelf != ""},
		})
		if err != nil {
			c.String(http.StatusInternalServerError, "Konnte Änderungen nicht speichern")
			return
		}
		c.Redirect(http.StatusFound, "/admin?tab=spiele")
	})

	r.POST("/api/items/delete", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		itemID, _ := strconv.Atoi(c.PostForm("id"))
		if c.PostForm("confirm_name") != c.PostForm("real_name") {
			c.String(http.StatusBadRequest, "Bestätigungsname stimmt nicht.")
			return
		}
		_ = queries.DeleteItem(context.Background(), int32(itemID))
		c.Redirect(http.StatusFound, "/admin?tab=spiele")
	})

	// NEU: Details Endpunkt für das Modal und den Kalender
	r.GET("/api/items/details", func(c *gin.Context) {
		idStr := c.Query("id")
		itemID, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige ID"})
			return
		}

		item, err := queries.GetItem(context.Background(), int32(itemID))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Spiel nicht gefunden"})
			return
		}

		bookings, err := queries.GetUpcomingBookingsForItem(context.Background(), int32(itemID))
		if err != nil {
			bookings = []db.GetUpcomingBookingsForItemRow{}
		}

		var images []string
		if item.CustomImageUrl.Valid && item.CustomImageUrl.String != "" {
			images = append(images, item.CustomImageUrl.String)
		}
		if item.ImageUrl.Valid && item.ImageUrl.String != "" {
			images = append(images, item.ImageUrl.String)
		}
		if len(images) == 0 {
			images = append(images, "/static/placeholder.png")
		}

		c.JSON(http.StatusOK, gin.H{
			"id":           item.ID,
			"name":         item.Name,
			"description":  item.Description.String,
			"min_players":  item.MinPlayers,
			"max_players":  item.MaxPlayers,
			"min_playtime": item.MinPlaytime,
			"max_playtime": item.MaxPlaytime,
			"status":       item.Status,
			"images":       images,
			"bookings":     bookings,
			"room":         item.Room.String,
			"shelf":        item.Shelf.String,
		})
	})

	// NEU: Historie laden (nur für Theke & Admins)
	r.GET("/api/items/history", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		idStr := c.Query("id")
		itemID, _ := strconv.Atoi(idStr)

		history, err := queries.GetItemHistory(context.Background(), int32(itemID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Konnte Historie nicht laden"})
			return
		}

		type HistoryResponse struct {
			ID        int32  `json:"id"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
			Status    string `json:"status"`
			Borrower  string `json:"borrower"`
		}

		var resp []HistoryResponse
		for _, h := range history {
			borrower := "Unbekannt"
			if h.GuestName.Valid && h.GuestName.String != "" {
				borrower = "Gast: " + h.GuestName.String
			} else if h.FirstName.Valid {
				borrower = h.FirstName.String + " " + h.LastName.String
			}

			resp = append(resp, HistoryResponse{
				ID:        h.ID,
				StartDate: h.StartDate.Format("02.01.2006"),
				EndDate:   h.EndDate.Format("02.01.2006"),
				Status:    string(h.Status),
				Borrower:  borrower,
			})
		}

		c.JSON(http.StatusOK, resp)
	})

	// ==========================================
	// API: USERS (NUTZER)
	// ==========================================
	r.POST("/api/users", RequireRole("admin"), func(c *gin.Context) {
		var req CreateUserRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
			return
		}

		role, oidcSub := req.Role, strings.TrimSpace(req.OidcSub)
		if role != "admin" && role != "maintainer" {
			role = "user"
		}
		if oidcSub == "" {
			oidcSub = "manual-" + strings.TrimSpace(req.Email)
		}

		_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
			FirstName: strings.TrimSpace(req.FirstName),
			LastName:  strings.TrimSpace(req.LastName),
			Email:     strings.TrimSpace(req.Email),
			Role:      db.UserRole(role),
			OidcSub:   oidcSub,
		})
		c.Redirect(http.StatusFound, "/admin?tab=nutzer")
	})

	r.POST("/api/users/update", RequireRole("admin"), func(c *gin.Context) {
		var req UpdateUserRequest
		if err := c.ShouldBind(&req); err != nil {
			c.String(http.StatusBadRequest, "Ungültige Eingabe")
			return
		}

		role, oidcSub := req.Role, strings.TrimSpace(req.OidcSub)
		if role != "admin" && role != "maintainer" {
			role = "user"
		}
		if oidcSub == "" {
			if existing, err := queries.GetUser(context.Background(), req.ID); err == nil {
				oidcSub = existing.OidcSub
			} else {
				oidcSub = "manual-" + strings.TrimSpace(req.Email)
			}
		}

		_ = queries.UpdateUser(context.Background(), db.UpdateUserParams{
			ID:        req.ID,
			FirstName: strings.TrimSpace(req.FirstName),
			LastName:  strings.TrimSpace(req.LastName),
			Email:     strings.TrimSpace(req.Email),
			Role:      db.UserRole(role),
			OidcSub:   oidcSub,
		})
		c.Redirect(http.StatusFound, "/admin?tab=nutzer")
	})

	r.POST("/api/users/delete", RequireRole("admin"), func(c *gin.Context) {
		userID, _ := strconv.Atoi(c.PostForm("id"))
		if c.PostForm("confirm_name") == c.PostForm("real_name") {
			_ = queries.DeleteUser(context.Background(), int32(userID))
		}
		c.Redirect(http.StatusFound, "/admin?tab=nutzer")
	})

	// ==========================================
	// API: BOARDGAMEGEEK (BGG)
	// ==========================================
	r.GET("/api/bgg/search", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Leerer Suchbegriff"})
			return
		}

		bggURL := "https://boardgamegeek.com/xmlapi2/search?type=boardgame&query=" + url.QueryEscape(query)
		enforceBGGCooldown()

		req, _ := http.NewRequest("GET", bggURL, nil)
		req.Header.Set("User-Agent", "FIUS-LeiheV2/1.0 (University of Stuttgart)")
		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG API Fehler"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var bggResult BGGSearchResponse
		if err := xml.Unmarshal(body, &bggResult); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "XML Parse Fehler"})
			return
		}
		c.JSON(http.StatusOK, bggResult.Items)
	})

	r.GET("/api/bgg/details", RequireRole("admin", "maintainer"), func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Keine ID"})
			return
		}

		bggURL := "https://boardgamegeek.com/xmlapi2/thing?id=" + id
		enforceBGGCooldown()

		req, _ := http.NewRequest("GET", bggURL, nil)
		req.Header.Set("User-Agent", "FIUS-LeiheV2/1.0 (University of Stuttgart)")
		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG API Fehler"})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var bggResult BGGThingResponse
		if err := xml.Unmarshal(body, &bggResult); err != nil || len(bggResult.Items) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "XML Parse Fehler oder nicht gefunden"})
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

	// ==========================================
	// START SERVER
	// ==========================================
	log.Println("🚀 FIUS Verleih-Backend startet auf http://localhost:8080")
	r.Run(":8080")
}
