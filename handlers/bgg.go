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

	"leiheV2/internal/bggutil"

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
	req.Header.Set("User-Agent", "LeiheV2/1.0")
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
		ID   string `xml:"id,attr" json:"ID"`
		Name struct {
			Value string `xml:"value,attr" json:"Value"`
		} `xml:"name" json:"Name"`
		Year struct {
			Value string `xml:"value,attr" json:"Value"`
		} `xml:"yearpublished" json:"Year"`
		Thumbnail string `xml:"-" json:"Thumbnail"`
	} `xml:"item"`
}

type BGGName struct {
	Value string `xml:"value,attr"`
	Type  string `xml:"type,attr"`
}

type BGGLink struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type BGGVersion struct {
	Names     []BGGName `xml:"name"`
	Image     string    `xml:"image"`
	Thumbnail string    `xml:"thumbnail"`
	Links     []BGGLink `xml:"link"`
}

type BGGThingItem struct {
	ID        string    `xml:"id,attr"`
	Names     []BGGName `xml:"name"`
	Image     string    `xml:"image"`
	Thumbnail string    `xml:"thumbnail"`
	Versions  struct {
		Items []BGGVersion `xml:"item"`
	} `xml:"versions"`
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
	Links      []BGGLink `xml:"link"`
	Statistics struct {
		Ratings struct {
			Average struct {
				Value string `xml:"value,attr"`
			} `xml:"average"`
		} `xml:"ratings"`
	} `xml:"statistics"`
}

type BGGThingResponse struct {
	Items []BGGThingItem `xml:"item"`
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

	if len(bggResult.Items) > 0 {
		ids := make([]string, 0, len(bggResult.Items))
		namesByID := make(map[string]string, len(bggResult.Items))
		for _, item := range bggResult.Items {
			ids = append(ids, item.ID)
			namesByID[item.ID] = item.Name.Value
		}
		thingsURL := "https://boardgamegeek.com/xmlapi2/thing?id=" + url.QueryEscape(strings.Join(ids, ",")) + "&versions=1"
		enforceBGGCooldown()
		thingsReq, err := newBGGRequest(http.MethodGet, thingsURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "BGG Bildanfrage konnte nicht erstellt werden"})
			return
		}
		thingsResp, err := client.Do(thingsReq)
		if err != nil || thingsResp.StatusCode != http.StatusOK {
			if thingsResp != nil {
				defer thingsResp.Body.Close()
			}
			respondWithBGGError(c, thingsResp, err)
			return
		}
		defer thingsResp.Body.Close()
		thingsBody, err := io.ReadAll(thingsResp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "BGG Bilder konnten nicht gelesen werden"})
			return
		}
		var things BGGThingResponse
		if err := xml.Unmarshal(thingsBody, &things); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "BGG Bilddaten konnten nicht verarbeitet werden"})
			return
		}
		imagesByID := make(map[string]string, len(things.Items))
		for _, item := range things.Items {
			version := selectBGGVersion(item, namesByID[item.ID])
			thumbnail := version.Thumbnail
			if thumbnail == "" {
				thumbnail = version.Image
			}
			if thumbnail == "" {
				thumbnail = item.Thumbnail
			}
			imagesByID[item.ID] = thumbnail
		}
		for i := range bggResult.Items {
			bggResult.Items[i].Thumbnail = imagesByID[bggResult.Items[i].ID]
		}
	}
	c.JSON(http.StatusOK, bggResult.Items)
}

func GetBGGDetails(c *gin.Context) {
	id := c.Query("id")
	selectedName := strings.TrimSpace(c.Query("name"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keine ID"})
		return
	}

	bggURL := "https://boardgamegeek.com/xmlapi2/thing?id=" + url.QueryEscape(id) + "&stats=1&versions=1"
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
	name := selectedName
	if name == "" {
		for _, candidate := range item.Names {
			if candidate.Type == "primary" {
				name = candidate.Value
				break
			}
		}
		if name == "" && len(item.Names) > 0 {
			name = item.Names[0].Value
		}
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

	var rating any
	if parsedRating, ok := bggutil.ParseRating(item.Statistics.Ratings.Average.Value); ok {
		rating = parsedRating
	}
	c.JSON(http.StatusOK, gin.H{
		"bgg_id":       item.ID,
		"name":         name,
		"image_url":    selectBGGVersionImage(item, selectedName),
		"min_players":  item.MinPlayers.Value,
		"max_players":  item.MaxPlayers.Value,
		"min_playtime": item.MinPlaytime.Value,
		"max_playtime": item.MaxPlaytime.Value,
		"bgg_rating":   rating,
		"categories":   categories,
		"is_expansion": isExpansion,
	})
}

func selectBGGVersionImage(item BGGThingItem, selectedName string) string {
	version := selectBGGVersion(item, selectedName)
	if version.Image != "" {
		return version.Image
	}
	return item.Image
}

func selectBGGVersion(item BGGThingItem, selectedName string) BGGVersion {
	var germanVersion BGGVersion
	for _, version := range item.Versions.Items {
		matchesName := false
		isGerman := false
		for _, link := range version.Links {
			if link.Type == "language" && strings.EqualFold(link.Value, "German") {
				isGerman = true
			}
		}
		for _, name := range version.Names {
			if name.Type == "primary" && selectedName != "" && strings.EqualFold(name.Value, selectedName) {
				matchesName = true
			}
		}
		if matchesName {
			return version
		}
		if selectedName != "" && isGerman && germanVersion.Image == "" && germanVersion.Thumbnail == "" {
			germanVersion = version
		}
	}
	if germanVersion.Image != "" || germanVersion.Thumbnail != "" {
		return germanVersion
	}
	return BGGVersion{}
}
