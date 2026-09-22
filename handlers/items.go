package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"leiheV2/db"

	"github.com/gin-gonic/gin"
)

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

func CreateItem(c *gin.Context) {
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
}

func UpdateItem(c *gin.Context) {
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
		if h.GuestName != nil && *h.GuestName != "" {
			borrower = "Gast: " + *h.GuestName
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
