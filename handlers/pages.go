package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"leiheV2/db"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func RenderIndex(c *gin.Context) {
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

	c.HTML(http.StatusOK, "index.html", pageData(c, gin.H{
		"TotalItems": len(items),
		"GameNames":  template.JS(namesJSON),
	}))
}

func RenderGamesCatalog(c *gin.Context) {
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
		image := "/static/images/image-placeholder.svg"
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

	c.HTML(http.StatusOK, "games.html", pageData(c, gin.H{
		"Games":      items,
		"Categories": categories,
		"GamesJSON":  string(gamesJSON),
	}))
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
	c.HTML(http.StatusOK, "admin_dashboard.html", pageData(c, gin.H{
		"ActiveTab": "dashboard",
		"Games":     items,
		"Users":     users,
	}))
}

func RenderAdminGames(c *gin.Context) {
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
	c.HTML(http.StatusOK, "admin_games.html", pageData(c, gin.H{
		"ActiveTab":  "games",
		"Games":      items,
		"Categories": categories,
	}))
}

func RenderAdminUsers(c *gin.Context) {
	users, err := queries.ListUsers(context.Background())
	if err != nil {
		c.String(http.StatusInternalServerError, "Datenbankfehler")
		return
	}
	c.HTML(http.StatusOK, "admin_users.html", pageData(c, gin.H{
		"ActiveTab": "users",
		"Users":     users,
	}))
}

func RenderAdminBookings(c *gin.Context) {
	bookings, err := queries.ListBookingOverview(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Buchungen konnten nicht geladen werden")
		return
	}
	pendingBookings := make([]db.BookingOverview, 0)
	for _, booking := range bookings {
		if booking.Status == "pending" {
			pendingBookings = append(pendingBookings, booking)
		}
	}
	c.HTML(http.StatusOK, "admin_bookings.html", pageData(c, gin.H{
		"ActiveTab":       "bookings",
		"Bookings":        bookings,
		"PendingBookings": pendingBookings,
		"Success":         c.Query("success"),
		"Error":           c.Query("error"),
	}))
}

func RenderAdminSettings(c *gin.Context) {
	settings, err := queries.GetAppSettings(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Einstellungen konnten nicht geladen werden")
		return
	}
	c.HTML(http.StatusOK, "admin_settings.html", pageData(c, gin.H{
		"ActiveTab": "settings",
		"Settings":  settings,
		"Success":   c.Query("success"),
		"Error":     c.Query("error"),
	}))
}

func RenderCheckoutPage(c *gin.Context) {
	session := sessions.Default(c)
	userRole := session.Get("user_role")
	if userRole == nil {
		userRole = "guest"
	}

	c.HTML(http.StatusOK, "checkout.html", pageData(c, gin.H{
		"UserRole": userRole,
	}))
}

func RenderUserSettings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	c.HTML(http.StatusOK, "user_settings.html", pageData(c, gin.H{
		"User":    user,
		"Success": c.Query("success"),
		"Error":   c.Query("error"),
	}))
}

func RenderLoginPage(c *gin.Context) {
	renderAuthPage(c, http.StatusOK, "")
}

func RenderRegisterPage(c *gin.Context) {
	if !auth.LocalAuth {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	renderAuthPage(c, http.StatusOK, "")
}

func renderAuthPage(c *gin.Context, status int, errorMessage string) {
	c.HTML(status, "auth.html", pageData(c, gin.H{
		"Error":       errorMessage,
		"LocalAuth":   auth.LocalAuth,
		"OIDCEnabled": auth.OIDCEnabled,
		"Register":    c.Request.URL.Path == "/register",
	}))
}

func currentUser(c *gin.Context) (db.User, bool) {
	session := sessions.Default(c)
	id, ok := session.Get("user_id").(int32)
	if !ok {
		return db.User{}, false
	}
	user, err := queries.GetUser(c.Request.Context(), id)
	return user, err == nil
}

func pageData(c *gin.Context, values gin.H) gin.H {
	session := sessions.Default(c)
	userRole := session.Get("user_role")
	if userRole == nil || userRole == "" {
		userRole = "guest"
	}
	values["UserRole"] = userRole
	if user, ok := currentUser(c); ok {
		values["UserName"] = strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
		values["CurrentUser"] = user
		values["CurrentUserIsOIDC"] = isOIDCUser(user)
	} else {
		values["UserName"] = ""
		values["CurrentUserIsOIDC"] = false
	}
	return values
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
