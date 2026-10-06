package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	thumbnailWidth  = 480
	thumbnailHeight = 288
	maxImageBytes   = 8 << 20
	maxImagePixels  = 12_000_000
	maxThumbCache   = 256
)

var (
	thumbnailCacheMu sync.Mutex
	thumbnailCache   = make(map[string][]byte)
	thumbnailCacheID []string
	thumbnailClient  = &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || req.URL.Scheme != "https" || !allowedImagePort(req.URL.Port()) || !isAllowedImageHost(req.URL.Hostname()) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
)

func GetItemThumbnail(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Query("id"))
	if err != nil || itemID <= 0 {
		c.Status(http.StatusBadRequest)
		return
	}

	item, err := queries.GetItem(context.Background(), int32(itemID))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	imageURL := ""
	if item.CustomImageUrl.Valid && item.CustomImageUrl.String != "" {
		imageURL = item.CustomImageUrl.String
	} else if item.ImageUrl.Valid {
		imageURL = item.ImageUrl.String
	}
	if imageURL == "" {
		c.Status(http.StatusNotFound)
		return
	}

	cacheKey := fmt.Sprintf("%d:%s", item.ID, imageURL)
	thumbnailCacheMu.Lock()
	thumbnail, cached := thumbnailCache[cacheKey]
	thumbnailCacheMu.Unlock()
	if !cached {
		source, err := readGameImage(imageURL)
		if err != nil {
			log.Printf("Could not read image for game %d: %v", item.ID, err)
			c.Status(http.StatusBadGateway)
			return
		}
		thumbnail, err = compressThumbnail(source)
		if err != nil {
			log.Printf("Could not create thumbnail for game %d: %v", item.ID, err)
			c.Status(http.StatusUnsupportedMediaType)
			return
		}
		thumbnailCacheMu.Lock()
		if len(thumbnailCacheID) >= maxThumbCache {
			delete(thumbnailCache, thumbnailCacheID[0])
			thumbnailCacheID = thumbnailCacheID[1:]
		}
		thumbnailCache[cacheKey] = thumbnail
		thumbnailCacheID = append(thumbnailCacheID, cacheKey)
		thumbnailCacheMu.Unlock()
	}

	etagBytes := sha256.Sum256(thumbnail)
	etag := `"` + hex.EncodeToString(etagBytes[:]) + `"`
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "image/jpeg", thumbnail)
}

func readGameImage(imageURL string) ([]byte, error) {
	if strings.HasPrefix(imageURL, "/static/custom_images/") {
		parsed, err := url.Parse(imageURL)
		if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid custom image path")
		}
		cleanPath := path.Clean(parsed.Path)
		if !strings.HasPrefix(cleanPath, "/static/custom_images/") {
			return nil, fmt.Errorf("invalid custom image path")
		}
		filename := filepath.Base(cleanPath)
		if filename == "." || filename == "/" {
			return nil, fmt.Errorf("invalid custom image path")
		}
		return readLimitedFile(filepath.Join("static", "custom_images", filename))
	}

	parsed, err := url.Parse(imageURL)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || !allowedImagePort(parsed.Port()) || !isAllowedImageHost(parsed.Hostname()) {
		return nil, fmt.Errorf("image host is not allowed")
	}
	request, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := thumbnailClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image server returned %s", response.Status)
	}
	return readLimited(response.Body)
}

func isAllowedImageHost(host string) bool {
	switch strings.ToLower(host) {
	case "cf.geekdo-images.com", "boardgamegeek.com", "www.boardgamegeek.com", "images.boardgamegeek.com":
		return true
	default:
		return false
	}
}

func allowedImagePort(port string) bool {
	return port == "" || port == "443"
}

func readLimitedFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLimited(file)
}

func readLimited(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds size limit")
	}
	return data, nil
}

func compressThumbnail(data []byte) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, fmt.Errorf("image dimensions exceed limit")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	sourceBounds := source.Bounds()
	sourceWidth, sourceHeight := sourceBounds.Dx(), sourceBounds.Dy()
	cropWidth, cropHeight := sourceWidth, sourceHeight
	if sourceWidth*thumbnailHeight > sourceHeight*thumbnailWidth {
		cropWidth = sourceHeight * thumbnailWidth / thumbnailHeight
	} else {
		cropHeight = sourceWidth * thumbnailHeight / thumbnailWidth
	}
	cropX := sourceBounds.Min.X + (sourceWidth-cropWidth)/2
	cropY := sourceBounds.Min.Y + (sourceHeight-cropHeight)/2

	outputWidth, outputHeight := thumbnailWidth, thumbnailHeight
	if cropWidth <= thumbnailWidth && cropHeight <= thumbnailHeight {
		outputWidth, outputHeight = cropWidth, cropHeight
	}
	resized := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	background := color.RGBA{R: 17, G: 24, B: 39, A: 255}
	for y := 0; y < outputHeight; y++ {
		for x := 0; x < outputWidth; x++ {
			sourceX := cropX + x*cropWidth/outputWidth
			sourceY := cropY + y*cropHeight/outputHeight
			pixel := color.NRGBAModel.Convert(source.At(sourceX, sourceY)).(color.NRGBA)
			alpha := uint32(pixel.A)
			resized.SetRGBA(x, y, color.RGBA{
				R: uint8((uint32(pixel.R)*alpha + uint32(background.R)*(255-alpha)) / 255),
				G: uint8((uint32(pixel.G)*alpha + uint32(background.G)*(255-alpha)) / 255),
				B: uint8((uint32(pixel.B)*alpha + uint32(background.B)*(255-alpha)) / 255),
				A: 255,
			})
		}
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, resized, &jpeg.Options{Quality: 76}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
