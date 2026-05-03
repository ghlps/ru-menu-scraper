package scraper

import (
	"log"

	"github.com/ghlps/poc-go-scraper/internal/models"
)

type scrapeState struct {
	currentMealType string
	mealOptions     []models.Meal
	payload         models.Menu
}

func (s *scrapeState) saveMeals() {
	if len(s.mealOptions) > 0 {
		log.Printf("Saving meals for: %s", s.currentMealType)
		s.payload.Meals[s.currentMealType] = s.mealOptions
		s.mealOptions = nil
	}
}
