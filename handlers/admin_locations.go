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

func RenderAdminLocations(c *gin.Context) {
	locations, err := queries.ListStorageShelves(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Regale konnten nicht geladen werden")
		return
	}
	locationsJSON, err := json.Marshal(locations)
	if err != nil {
		c.String(http.StatusInternalServerError, "Regale konnten nicht geladen werden")
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
	isKnown, err := queries.HasStorageShelf(c.Request.Context(), location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Regale konnten nicht geladen werden"})
		return
	}
	if !isKnown {
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

	err = queries.InTx(c.Request.Context(), func(tx *db.Queries) error {
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

type fillMissingRoomsRequest struct {
	ItemIDs []int32 `json:"item_ids" binding:"required"`
	Room    string  `json:"room" binding:"required"`
}

func FillMissingLocationRooms(c *gin.Context) {
	var req fillMissingRoomsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
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
			if err := tx.UpdateItemRoomIfMissing(c.Request.Context(), id, room); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mindestens ein Spiel hat bereits einen Raum oder kein gespeichertes Fach"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Räume konnten nicht gespeichert werden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": len(uniqueIDs)})
}

type addStorageShelfRequest struct {
	Code string `json:"code" binding:"required"`
}

func AddStorageShelf(c *gin.Context) {
	var req addStorageShelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
		return
	}
	code, err := db.ValidateStorageShelfCode(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Der Regalname muss zwischen 1 und 100 Zeichen lang sein"})
		return
	}
	exists, err := queries.HasStorageShelf(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Regale konnten nicht geprüft werden"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Dieses Regal existiert bereits"})
		return
	}
	if err := queries.AddStorageShelf(c.Request.Context(), code); err != nil {
		if errors.Is(err, db.ErrStorageShelfAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Dieses Regal existiert bereits"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Regal konnte nicht hinzugefügt werden"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": code})
}

type renameStorageShelfRequest struct {
	OldCode string `json:"old_code" binding:"required"`
	NewCode string `json:"new_code" binding:"required"`
}

func RenameStorageShelf(c *gin.Context) {
	var req renameStorageShelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
		return
	}
	oldCode := strings.TrimSpace(req.OldCode)
	newCode, err := db.ValidateStorageShelfCode(req.NewCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Der Regalname muss zwischen 1 und 100 Zeichen lang sein"})
		return
	}
	exists, err := queries.HasStorageShelf(c.Request.Context(), oldCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Regale konnten nicht geprüft werden"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Das Regal wurde nicht gefunden"})
		return
	}
	if oldCode == newCode {
		c.JSON(http.StatusOK, gin.H{"old_code": oldCode, "new_code": newCode, "updated": 0})
		return
	}
	if !strings.EqualFold(oldCode, newCode) {
		exists, err = queries.HasStorageShelf(c.Request.Context(), newCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Regale konnten nicht geprüft werden"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "Dieses Regal existiert bereits"})
			return
		}
	}
	if err := queries.RenameStorageShelf(c.Request.Context(), oldCode, newCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Das Regal wurde nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Regal konnte nicht umbenannt werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"old_code": oldCode, "new_code": newCode})
}
