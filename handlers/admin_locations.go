package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"leiheV2/db"

	"github.com/gin-gonic/gin"
)

var gameStorageLocations = []string{
	"A1L", "A1R", "A2L", "A2R", "A3L", "A3R", "A4L", "A4R",
	"B0", "B2L", "B2R", "B3L", "B3R", "B4L", "B4R",
	"C1L", "C1R", "C2L", "C2R", "C3L", "C3R", "CU",
}

func RenderAdminLocations(c *gin.Context) {
	locationsJSON, err := json.Marshal(gameStorageLocations)
	if err != nil {
		c.String(http.StatusInternalServerError, "Lagerorte konnten nicht geladen werden")
		return
	}
	rooms, err := queries.ListStorageRooms(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Räume konnten nicht geladen werden")
		return
	}
	roomsJSON, err := json.Marshal(rooms)
	if err != nil {
		c.String(http.StatusInternalServerError, "Räume konnten nicht geladen werden")
		return
	}
	c.HTML(http.StatusOK, "admin_locations.html", pageData(c, gin.H{
		"ActiveTab":     "locations",
		"LocationsJSON": string(locationsJSON),
		"RoomsJSON":     string(roomsJSON),
		"SelectedRoom":  strings.TrimSpace(c.Query("room")),
	}))
}

func ListLocationItems(c *gin.Context) {
	items, err := queries.ListItemsWithCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Spiele konnten nicht geladen werden"})
		return
	}

	type locationItem struct {
		ID    int32  `json:"id"`
		Name  string `json:"name"`
		Room  string `json:"room"`
		Shelf string `json:"shelf"`
	}
	result := make([]locationItem, 0, len(items))
	for _, item := range items {
		result = append(result, locationItem{
			ID:    item.ID,
			Name:  item.Name,
			Room:  item.Room.String,
			Shelf: item.Shelf.String,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

type assignLocationRequest struct {
	ItemIDs  []int32 `json:"item_ids" binding:"required"`
	Room     string  `json:"room" binding:"required"`
	Location string  `json:"location" binding:"required"`
}

func AssignLocationItems(c *gin.Context) {
	var req assignLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
		return
	}

	location := strings.TrimSpace(req.Location)
	if !isKnownStorageLocation(location) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unbekannter Lagerort"})
		return
	}
	room := strings.TrimSpace(req.Room)
	if room == "" || len(room) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bitte gib einen gültigen Raum an (maximal 100 Zeichen)"})
		return
	}
	if len(req.ItemIDs) == 0 || len(req.ItemIDs) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bitte wähle zwischen 1 und 500 Spielen aus"})
		return
	}

	uniqueIDs := make([]int32, 0, len(req.ItemIDs))
	seen := make(map[int32]struct{}, len(req.ItemIDs))
	for _, id := range req.ItemIDs {
		if id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Spiele-ID"})
			return
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	err := queries.InTx(c.Request.Context(), func(tx *db.Queries) error {
		for _, id := range uniqueIDs {
			if _, err := tx.GetItem(c.Request.Context(), id); err != nil {
				return err
			}
			if err := tx.UpdateItemLocation(c.Request.Context(), id, room, location); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mindestens ein ausgewähltes Spiel existiert nicht"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lagerorte konnten nicht gespeichert werden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": len(uniqueIDs)})
}

func isKnownStorageLocation(location string) bool {
	for _, known := range gameStorageLocations {
		if location == known {
			return true
		}
	}
	return false
}
