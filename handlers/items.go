package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"leiheV2/db"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type CreateItemRequest struct {
	Name        string   `form:"name" binding:"required"`
	MinPlayers  int32    `form:"min_players"`
	MaxPlayers  int32    `form:"max_players"`
	MinPlaytime int32    `form:"min_playtime"`
	MaxPlaytime int32    `form:"max_playtime"`
	Room        string   `form:"room"`
	Shelf       string   `form:"shelf"`
	Categories  []string `form:"categories"`
}

type UpdateItemRequest struct {
	ID          int32    `form:"id" binding:"required"`
	Name        string   `form:"name" binding:"required"`
	MinPlayers  int32    `form:"min_players"`
	MaxPlayers  int32    `form:"max_players"`
	MinPlaytime int32    `form:"min_playtime"`
	MaxPlaytime int32    `form:"max_playtime"`
	Room        string   `form:"room"`
	Shelf       string   `form:"shelf"`
	Categories  []string `form:"categories"`
}

func validateItemDimensions(minPlayers, maxPlayers, minPlaytime, maxPlaytime int32) error {
	if minPlayers < 0 || maxPlayers < 0 || minPlaytime < 0 || maxPlaytime < 0 {
		return fmt.Errorf("Spielerzahlen und Spielzeiten dürfen nicht negativ sein")
	}
	if minPlayers > maxPlayers {
		return fmt.Errorf("Die minimale Spielerzahl darf die maximale nicht überschreiten")
	}
	if minPlaytime > maxPlaytime {
		return fmt.Errorf("Die minimale Spielzeit darf die maximale nicht überschreiten")
	}
	return nil
}

func renderAdminGamesError(c *gin.Context, status int, message string) {
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
	session := sessions.Default(c)
	c.HTML(status, "admin_games.html", gin.H{
		"ActiveTab":  "games",
		"Games":      items,
		"Categories": categories,
		"UserRole":   session.Get("user_role"),
		"Error":      message,
	})
}

func normalizeCategories(categories []string) []string {
	seen := make(map[string]struct{}, len(categories))
	result := make([]string, 0, len(categories))
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		key := strings.ToLower(category)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, category)
	}
	return result
}

func saveItemCategories(ctx context.Context, q *db.Queries, itemID int32, categories []string) error {
	if err := q.ClearItemCategories(ctx, itemID); err != nil {
		return err
	}
	for _, name := range normalizeCategories(categories) {
		category, err := q.GetOrCreateCategory(ctx, name)
		if err != nil {
			return err
		}
		if err := q.AddItemCategory(ctx, db.AddItemCategoryParams{ItemID: itemID, CategoryID: category.ID}); err != nil {
			return err
		}
	}
	return nil
}

func CreateItem(c *gin.Context) {
	var req CreateItemRequest
	if err := c.ShouldBind(&req); err != nil {
		renderAdminGamesError(c, http.StatusBadRequest, "Ungültige Eingabe: "+err.Error())
		return
	}
	if err := validateItemDimensions(req.MinPlayers, req.MaxPlayers, req.MinPlaytime, req.MaxPlaytime); err != nil {
		renderAdminGamesError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := queries.InTx(context.Background(), func(qTx *db.Queries) error {
		item, err := qTx.CreateItem(context.Background(), db.CreateItemParams{
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
			return err
		}
		return saveItemCategories(context.Background(), qTx, item.ID, req.Categories)
	})
	if err != nil {
		renderAdminGamesError(c, http.StatusInternalServerError, "Konnte Item nicht speichern")
		return
	}
	c.Redirect(http.StatusFound, "/admin/games")
}

func UpdateItem(c *gin.Context) {
	var req UpdateItemRequest
	if err := c.ShouldBind(&req); err != nil {
		renderAdminGamesError(c, http.StatusBadRequest, "Ungültige Eingabe: "+err.Error())
		return
	}
	if err := validateItemDimensions(req.MinPlayers, req.MaxPlayers, req.MinPlaytime, req.MaxPlaytime); err != nil {
		renderAdminGamesError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := queries.InTx(context.Background(), func(qTx *db.Queries) error {
		if err := qTx.UpdateItem(context.Background(), db.UpdateItemParams{
			ID:          req.ID,
			Name:        req.Name,
			MinPlayers:  req.MinPlayers,
			MaxPlayers:  req.MaxPlayers,
			MinPlaytime: req.MinPlaytime,
			MaxPlaytime: req.MaxPlaytime,
			Room:        sql.NullString{String: req.Room, Valid: req.Room != ""},
			Shelf:       sql.NullString{String: req.Shelf, Valid: req.Shelf != ""},
		}); err != nil {
			return err
		}
		return saveItemCategories(context.Background(), qTx, req.ID, req.Categories)
	})
	if err != nil {
		renderAdminGamesError(c, http.StatusInternalServerError, "Konnte Änderungen nicht speichern")
		return
	}
	c.Redirect(http.StatusFound, "/admin/games")
}

func CreateCategory(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		renderAdminGamesError(c, http.StatusBadRequest, "Der Kategoriename darf nicht leer sein")
		return
	}
	if _, err := queries.GetOrCreateCategory(context.Background(), name); err != nil {
		renderAdminGamesError(c, http.StatusInternalServerError, "Kategorie konnte nicht gespeichert werden")
		return
	}
	c.Redirect(http.StatusFound, "/admin/games")
}

func DeleteCategory(c *gin.Context) {
	categoryID, err := strconv.Atoi(c.PostForm("id"))
	if err != nil {
		renderAdminGamesError(c, http.StatusBadRequest, "Ungültige Kategorie")
		return
	}
	if err := queries.DeleteCategory(context.Background(), int32(categoryID)); err != nil {
		renderAdminGamesError(c, http.StatusInternalServerError, "Kategorie konnte nicht gelöscht werden")
		return
	}
	c.Redirect(http.StatusFound, "/admin/games")
}

func DeleteItem(c *gin.Context) {
	itemID, _ := strconv.Atoi(c.PostForm("id"))
	if c.PostForm("confirm_name") != c.PostForm("real_name") {
		c.String(http.StatusBadRequest, "Bestätigungsname stimmt nicht.")
		return
	}
	_ = queries.DeleteItem(context.Background(), int32(itemID))
	c.Redirect(http.StatusFound, "/admin?tab=spiele")
}

func UpdateItemImage(c *gin.Context) {
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
}

func GetItemDetails(c *gin.Context) {
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
}

func GetItemHistory(c *gin.Context) {
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
}
