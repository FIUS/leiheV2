package handlers

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

const bggRequestCooldown = 5 * time.Second

var bggMutex sync.Mutex
var bggLastCall time.Time

func enforceBGGCooldown() {
	bggMutex.Lock()
	defer bggMutex.Unlock()

	elapsed := time.Since(bggLastCall)
	if elapsed < bggRequestCooldown {
		time.Sleep(bggRequestCooldown - elapsed)
	}
	bggLastCall = time.Now()
}

func newBGGRequest(method, requestURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FIUS-LeiheV2/1.0 (University of Stuttgart)")
	if token := strings.TrimSpace(os.Getenv("BGG_API_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func respondWithBGGError(c *gin.Context, resp *http.Response, requestErr error) {
	if requestErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "BGG API nicht erreichbar"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error":  "BGG API Fehler",
		"status": resp.StatusCode,
		"detail": message,
	})
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
		ID    string `xml:"id,attr"`
		Names []struct {
			Value string `xml:"value,attr"`
			Type  string `xml:"type,attr"`
		} `xml:"name"`
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

func normalizeBGGSearchQuery(query string) string {
	var builder strings.Builder
	lastSpace := false
	for _, r := range strings.TrimSpace(query) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func SearchBGG(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Leerer Suchbegriff"})
		return
	}

	query = normalizeBGGSearchQuery(query)
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiger Suchbegriff"})
		return
	}
	bggURL := "https://boardgamegeek.com/xmlapi2/search?type=boardgame&query=" + url.QueryEscape(query)
	enforceBGGCooldown()

	req, err := newBGGRequest(http.MethodGet, bggURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG Anfrage konnte nicht erstellt werden"})
		return
	}
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			defer resp.Body.Close()
		}
		respondWithBGGError(c, resp, err)
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

	req, err := newBGGRequest(http.MethodGet, bggURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG Anfrage konnte nicht erstellt werden"})
		return
	}
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			defer resp.Body.Close()
		}
		respondWithBGGError(c, resp, err)
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
	name := ""
	for _, candidate := range item.Names {
		if candidate.Type == "primary" {
			name = candidate.Value
			break
		}
	}
	if name == "" && len(item.Names) > 0 {
		name = item.Names[0].Value
	}
	var categories []string
	isExpansion := false
	for _, link := range item.Links {
		if link.Type == "boardgamecategory" {
			categories = append(categories, link.Value)
		}
		if link.Type == "boardgameexpansion" {
			isExpansion = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"bgg_id":       item.ID,
		"name":         name,
		"image_url":    item.Image,
		"min_players":  item.MinPlayers.Value,
		"max_players":  item.MaxPlayers.Value,
		"min_playtime": item.MinPlaytime.Value,
		"max_playtime": item.MaxPlaytime.Value,
		"categories":   categories,
		"is_expansion": isExpansion,
	})
}
