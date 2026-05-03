package scraper

import (
	"log"
	"slices"

	"github.com/ghlps/poc-go-scraper/internal/models"
)

func mealsByName(meals []models.Meal) map[string]models.Meal {
	idx := make(map[string]models.Meal, len(meals))
	for _, m := range meals {
		idx[m.Name] = m
	}
	return idx
}

func isMealChanged(prev, curr models.Meal) bool {
	if prev.Name != curr.Name {
		return true
	}
	if !slices.Equal(prev.Icons, curr.Icons) {
		return true
	}
	return false
}

func markChangedMeals(previous, current *models.Menu) {
	for mealType, currentMeals := range current.Meals {
		previousMeals, existed := previous.Meals[mealType]
		if !existed {
			for i := range currentMeals {
				currentMeals[i].Changed = true
			}
			current.Meals[mealType] = currentMeals
			log.Printf("Detected NEW meal type: %s with %d meals", mealType, len(currentMeals))
			continue
		}

		prevIdx := mealsByName(previousMeals)
		changed := false
		for i, meal := range currentMeals {
			prevMeal, existed := prevIdx[meal.Name]
			if !existed || isMealChanged(prevMeal, meal) {
				currentMeals[i].Changed = true
				changed = true
			}
		}
		if changed {
			current.Meals[mealType] = currentMeals
			log.Printf("Detected CHANGED meal type: %s", mealType)
		}
	}

	for mealType := range previous.Meals {
		if _, exists := current.Meals[mealType]; !exists {
			log.Printf("Detected REMOVED meal type: %s", mealType)
		}
	}
}
