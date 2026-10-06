package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

const (
	ThumbnailWidth  = 480
	ThumbnailHeight = 288
	maxImagePixels  = 20_000_000
)

func IsAllowedHost(host string) bool {
	switch strings.ToLower(host) {
	case "cf.geekdo-images.com", "boardgamegeek.com", "www.boardgamegeek.com", "images.boardgamegeek.com":
		return true
	default:
		return false
	}
}

func CompressThumbnail(data []byte) ([]byte, error) {
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
	scale := math.Min(float64(ThumbnailWidth)/float64(sourceWidth), float64(ThumbnailHeight)/float64(sourceHeight))
	if scale > 1 {
		scale = 1
	}
	outputWidth := max(1, int(float64(sourceWidth)*scale))
	outputHeight := max(1, int(float64(sourceHeight)*scale))
	resized := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	background := color.RGBA{R: 17, G: 24, B: 39, A: 255}
	for y := 0; y < outputHeight; y++ {
		for x := 0; x < outputWidth; x++ {
			sourceX := sourceBounds.Min.X + x*sourceWidth/outputWidth
			sourceY := sourceBounds.Min.Y + y*sourceHeight/outputHeight
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
