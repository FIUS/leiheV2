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
	namesJSON, _ := json.Marshal(names)

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
	categories, _ := queries.ListCategories(context.Background())

	c.HTML(http.StatusOK, "games.html", gin.H{
		"Games":      items,
		"Categories": categories,
		"UserRole":   session.Get("user_role"),
	})
}

func RenderAdmin(c *gin.Context) {
	session := sessions.Default(c)
	items, _ := queries.ListItems(context.Background())
	users, _ := queries.ListUsers(context.Background())

	c.HTML(http.StatusOK, "managment.html", gin.H{
		"Games":    items,
		"Users":    users,
		"UserRole": session.Get("user_role"),
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
}
