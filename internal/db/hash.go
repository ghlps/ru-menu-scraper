package db

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ghlps/poc-go-scraper/internal/models"
)

func HashMenu(menu *models.Menu) (string, error) {
	if menu == nil {
		return "", fmt.Errorf("menu is nil")
	}

	meals := make(map[string][]models.Meal, len(menu.Meals))
	for mealType, items := range menu.Meals {
		stripped := make([]models.Meal, len(items))
		for i, m := range items {
			m.Changed = false
			stripped[i] = m
		}
		meals[mealType] = stripped
	}

	menuCopy := *menu
	menuCopy.Meals = meals

	b, err := json.Marshal(menuCopy)
	if err != nil {
		return "", fmt.Errorf("hash marshal failed: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
