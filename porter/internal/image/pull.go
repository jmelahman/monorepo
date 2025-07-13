package image

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/jmelahman/porter/internal/paths"
)

func PullImage(ref string) (string, error) {
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

	// Skip if already exists
	imageDir := filepath.Join(paths.ImageDir(), imageID)
	rootfsDir := filepath.Join(paths.RootfsDir(), imageID)
	if dirExists(imageDir) && dirExists(rootfsDir) {
		return imageID, nil
	}

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

	return imageID, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
