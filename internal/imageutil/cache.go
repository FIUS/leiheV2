package imageutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
)

type ThumbnailCache struct {
	dir string
}

func NewThumbnailCache(dir string) ThumbnailCache {
	return ThumbnailCache{dir: dir}
}

func (cache ThumbnailCache) Get(key string) ([]byte, bool, error) {
	data, err := os.ReadFile(cache.filename(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
		if removeErr := os.Remove(cache.filename(key)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, false, fmt.Errorf("invalid cached thumbnail (%v), remove cache file: %w", err, removeErr)
		}
		return nil, false, fmt.Errorf("invalid cached thumbnail: %w", err)
	}
	return data, true, nil
}

func (cache ThumbnailCache) Put(key string, data []byte) error {
	if err := os.MkdirAll(cache.dir, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(cache.dir, ".thumbnail-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, cache.filename(key))
}

func (cache ThumbnailCache) filename(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(cache.dir, hex.EncodeToString(hash[:])+".jpg")
}
