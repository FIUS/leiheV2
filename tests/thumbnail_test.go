package tests

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"path/filepath"
	"testing"

	"leiheV2/internal/imageutil"
)

func TestThumbnailCachePersistsAcrossInstances(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	first := imageutil.NewThumbnailCache(cacheDir)
	key := "42:https://cf.geekdo-images.com/game.jpg"
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatalf("encode thumbnail: %v", err)
	}
	expected := encoded.Bytes()

	if _, found, err := first.Get(key); err != nil || found {
		t.Fatalf("initial cache lookup = (found %v, err %v), want (false, nil)", found, err)
	}
	if err := first.Put(key, expected); err != nil {
		t.Fatalf("persist thumbnail: %v", err)
	}

	reopened := imageutil.NewThumbnailCache(cacheDir)
	got, found, err := reopened.Get(key)
	if err != nil {
		t.Fatalf("read persisted thumbnail: %v", err)
	}
	if !found {
		t.Fatal("expected a persisted thumbnail to be found by a new cache instance")
	}
	if !bytes.Equal(got, expected) {
		t.Fatal("persisted thumbnail bytes differ from the stored bytes")
	}
	files, err := filepath.Glob(filepath.Join(cacheDir, "*.jpg"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one hashed cache file, found %d (err %v)", len(files), err)
	}
	if filepath.Base(files[0]) == key+".jpg" {
		t.Fatal("cache filename should not contain the raw key")
	}
}

func TestCompressThumbnailResizesAndEncodesJPEG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1200, 800))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode source image: %v", err)
	}

	output, err := imageutil.CompressThumbnail(input.Bytes())
	if err != nil {
		t.Fatalf("compress thumbnail: %v", err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if config.Width > imageutil.ThumbnailWidth || config.Height > imageutil.ThumbnailHeight {
		t.Fatalf("thumbnail dimensions = %dx%d, exceeds maximum %dx%d", config.Width, config.Height, imageutil.ThumbnailWidth, imageutil.ThumbnailHeight)
	}
	if config.Width*2 != config.Height*3 {
		t.Fatalf("thumbnail dimensions = %dx%d, want the full source aspect ratio 3:2", config.Width, config.Height)
	}
	if len(output) >= input.Len() {
		t.Fatalf("thumbnail size = %d bytes, source size = %d bytes; want a smaller image", len(output), input.Len())
	}
}

func TestCompressThumbnailAcceptsHighResolutionJPEG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4864, 3648))
	var input bytes.Buffer
	if err := jpeg.Encode(&input, source, &jpeg.Options{Quality: 75}); err != nil {
		t.Fatalf("encode source image: %v", err)
	}

	output, err := imageutil.CompressThumbnail(input.Bytes())
	if err != nil {
		t.Fatalf("compress high-resolution thumbnail: %v", err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(output))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if config.Width != 384 || config.Height != 288 {
		t.Fatalf("thumbnail dimensions = %dx%d, want 384x288 to preserve the source aspect ratio", config.Width, config.Height)
	}
}

func TestIsAllowedImageHost(t *testing.T) {
	if !imageutil.IsAllowedHost("cf.geekdo-images.com") {
		t.Fatal("expected BoardGameGeek image host to be allowed")
	}
	if imageutil.IsAllowedHost("127.0.0.1") {
		t.Fatal("unexpected host allowed")
	}
}
