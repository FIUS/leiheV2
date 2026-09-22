package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func RenderIndex(c *gin.Context) {
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
	namesJSON, err := json.Marshal(names)
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"TotalItems": len(items),
		"GameNames":  template.JS(namesJSON),
		"UserRole":   session.Get("user_role"),
	})
}

func RenderGamesCatalog(c *gin.Context) {
	session := sessions.Default(c)
	items, err := queries.ListItemsWithCategories(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	categories, err := queries.ListCategories(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	type gameCatalogItem struct {
		ID    int32  `json:"id"`
		Name  string `json:"name"`
		Image string `json:"image"`
	}
	games := make([]gameCatalogItem, 0, len(items))
	for _, item := range items {
		image := ""
		if item.CustomImageUrl.Valid {
			image = item.CustomImageUrl.String
		} else if item.ImageUrl.Valid {
			image = item.ImageUrl.String
		}
		games = append(games, gameCatalogItem{
			ID:    item.ID,
			Name:  item.Name,
			Image: image,
		})
	}
	gamesJSON, err := json.Marshal(games)
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}

	c.HTML(http.StatusOK, "games.html", gin.H{
		"Games":      items,
		"Categories": categories,
		"UserRole":   session.Get("user_role"),
		"GamesJSON":  string(gamesJSON),
	})
}

func RenderAdminDashboard(c *gin.Context) {
	items, err := queries.ListItems(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	users, err := queries.ListUsers(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	session := sessions.Default(c)

	c.HTML(http.StatusOK, "admin_dashboard.html", gin.H{
		"ActiveTab": "dashboard",
		"Games":     items,
		"Users":     users,
		"UserRole":  session.Get("user_role"),
	})
}

func RenderAdminGames(c *gin.Context) {
	items, err := queries.ListItems(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	session := sessions.Default(c)

	c.HTML(http.StatusOK, "admin_games.html", gin.H{
		"ActiveTab": "games",
		"Games":     items,
		"UserRole":  session.Get("user_role"),
	})
}

func RenderAdminUsers(c *gin.Context) {
	users, err := queries.ListUsers(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	session := sessions.Default(c)

	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"ActiveTab": "users",
		"Users":     users,
		"UserRole":  session.Get("user_role"),
	})
}

func RenderAdminBookings(c *gin.Context) {
	session := sessions.Default(c)
	c.HTML(http.StatusOK, "admin_bookings.html", gin.H{
		"ActiveTab": "bookings",
		"UserRole":  session.Get("user_role"),
	})
}

func RenderAdminSettings(c *gin.Context) {
	session := sessions.Default(c)
	c.HTML(http.StatusOK, "admin_settings.html", gin.H{
		"ActiveTab": "settings",
		"UserRole":  session.Get("user_role"),
	})
}

func RenderCheckoutPage(c *gin.Context) {
	session := sessions.Default(c)
	userRole := session.Get("user_role")
	if userRole == nil {
		userRole = "guest"
	}
	c.HTML(http.StatusOK, "checkout.html", gin.H{
		"UserRole": userRole,
	})
}

func RenderSetupBilder(c *gin.Context) {
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

	games, err := queries.ListItems(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
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
}
