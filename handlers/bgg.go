package handlers

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var bggMutex sync.Mutex
var bggLastCall time.Time

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

func SearchBGG(c *gin.Context) {
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
}

func GetBGGDetails(c *gin.Context) {
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
}
