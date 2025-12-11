package image

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/jmelahman/runtainer/internal/paths"
)

type imageCache struct {
	RefToID map[string]string `json:"ref_to_id"`
}

func getCacheFilePath() string {
	return filepath.Join(paths.StateDir(), "image_cache.json")
}

func loadImageCache() (*imageCache, error) {
	cachePath := getCacheFilePath()
	cache := &imageCache{RefToID: make(map[string]string)}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, err
	}

	err = json.Unmarshal(data, cache)
	if err != nil {
		return nil, err
	}

	return cache, nil
}

func saveImageCache(cache *imageCache) error {
	cachePath := getCacheFilePath()
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(cachePath), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func PullImage(ref string) (string, error) {
	// Check local cache first
	cache, err := loadImageCache()
	if err != nil {
		return "", fmt.Errorf("failed to load image cache: %w", err)
	}

	if imageID, exists := cache.RefToID[ref]; exists {
		// Check if the image files still exist
		imageDir := filepath.Join(paths.ImageDir(), imageID)
		rootfsDir := filepath.Join(paths.RootfsDir(), imageID)
		if dirExists(imageDir) && dirExists(rootfsDir) {
			fmt.Println("Found cached image")
			return imageID, nil
		}
		// Image files missing, remove from cache
		delete(cache.RefToID, ref)
	}

	// Image not in cache or files missing, pull from registry
	descriptor, err := crane.Get(ref)
	if err != nil {
		return "", err
	}

	image, err := descriptor.Image()
	if err != nil {
		return "", err
	}

	manifest, err := image.Manifest()
	if err != nil {
		return "", err
	}
	imageID := manifest.Config.Digest.Hex[:12]

	// Skip if already exists (in case another process pulled it concurrently)
	imageDir := filepath.Join(paths.ImageDir(), imageID)
	rootfsDir := filepath.Join(paths.RootfsDir(), imageID)
	if dirExists(imageDir) && dirExists(rootfsDir) {
		fmt.Println("Found cached image")
	} else {
		if err := os.MkdirAll(imageDir, 0755); err != nil {
			return "", err
		}
		if err := os.MkdirAll(rootfsDir, 0755); err != nil {
			return "", err
		}

		// Save config.json
		cfg, err := image.ConfigFile()
		if err != nil {
			return "", err
		}
		cfgData, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(filepath.Join(imageDir, "config.json"), cfgData, 0644); err != nil {
			return "", err
		}

		// Save manifest.json
		manifestData, _ := json.MarshalIndent(manifest, "", "  ")
		if err := os.WriteFile(filepath.Join(imageDir, "manifest.json"), manifestData, 0644); err != nil {
			return "", err
		}

		// Extract rootfs
		layers, err := image.Layers()
		if err != nil {
			return "", err
		}
		for i, layer := range layers {
			r, err := layer.Uncompressed()
			if err != nil {
				return "", fmt.Errorf("layer %d: %w", i, err)
			}
			if err := untarInto(r, rootfsDir); err != nil {
				return "", fmt.Errorf("extract layer %d: %w", i, err)
			}
		}
	}

	// Update cache with new mapping
	cache.RefToID[ref] = imageID
	if err := saveImageCache(cache); err != nil {
		return "", fmt.Errorf("failed to save image cache: %w", err)
	}

	return imageID, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
