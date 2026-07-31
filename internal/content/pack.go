package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anicolao/arknova/internal/game"
)

type manifest struct {
	Version string      `json:"version"`
	Cards   []game.Card `json:"cards"`
	Assets  []asset     `json:"assets"`
}

type asset struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

func Load(root string) (game.Catalog, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return game.Catalog{}, fmt.Errorf("read content manifest: %w", err)
	}
	var pack manifest
	if err := json.Unmarshal(data, &pack); err != nil {
		return game.Catalog{}, fmt.Errorf("decode content manifest: %w", err)
	}
	if pack.Version == "" || len(pack.Cards) == 0 {
		return game.Catalog{}, errors.New("content manifest requires a version and cards")
	}
	assets := make(map[string]asset, len(pack.Assets))
	for _, entry := range pack.Assets {
		if entry.Path == "" || entry.MediaType == "" || entry.Width <= 0 || entry.Height <= 0 {
			return game.Catalog{}, errors.New("every content asset requires path, media type, width, and height")
		}
		if filepath.IsAbs(entry.Path) || strings.HasPrefix(filepath.Clean(entry.Path), "..") {
			return game.Catalog{}, fmt.Errorf("asset path %q escapes the content pack", entry.Path)
		}
		if _, err := os.Stat(filepath.Join(root, entry.Path)); err != nil {
			return game.Catalog{}, fmt.Errorf("content asset %q: %w", entry.Path, err)
		}
		assets[entry.Path] = entry
	}
	seen := make(map[string]bool, len(pack.Cards))
	for _, card := range pack.Cards {
		if card.ID == "" || card.Name == "" || card.Image == "" {
			return game.Catalog{}, errors.New("every content card requires id, name, and image")
		}
		if seen[card.ID] {
			return game.Catalog{}, fmt.Errorf("duplicate card id %q", card.ID)
		}
		seen[card.ID] = true
		if _, ok := assets[card.Image]; !ok {
			return game.Catalog{}, fmt.Errorf("card %q references undeclared asset %q", card.ID, card.Image)
		}
	}
	return game.Catalog{Version: pack.Version, Cards: pack.Cards}, nil
}
