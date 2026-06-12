package scraper

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ghlps/poc-go-scraper/internal/models"
)

type scrapeState struct {
	ctx             context.Context
	s3Client        *s3.Client
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
