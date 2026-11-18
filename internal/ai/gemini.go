package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"

	"github.com/ghlps/poc-go-scraper/internal/models"
	"google.golang.org/genai"
)

type MealInd struct {
	Meals map[string][]models.Meal `json:"meals"`
}

func menuWithoutCalories(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var menu map[string]any
	if err := decoder.Decode(&menu); err != nil {
		return nil, fmt.Errorf("decode menu JSON: %w", err)
	}

	if menu == nil {
		return nil, fmt.Errorf("menu JSON must be an object")
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected data after menu JSON")
		}
		return nil, fmt.Errorf("invalid data after menu JSON: %w", err)
	}

	for mealType, value := range menu {
		meals, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("meal type %q must contain an array", mealType)
		}

		for i, value := range meals {
			meal, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("meal %d in %q must be an object", i, mealType)
			}
			delete(meal, "calories")
		}
	}

	return menu, nil
}

func validateCalorieResponse(original, response []byte) error {
	originalMenu, err := menuWithoutCalories(original)
	if err != nil {
		return fmt.Errorf("validate original menu: %w", err)
	}

	responseMenu, err := menuWithoutCalories(response)
	if err != nil {
		return fmt.Errorf("validate generated menu: %w", err)
	}

	if !reflect.DeepEqual(originalMenu, responseMenu) {
		return fmt.Errorf("generated menu changed fields other than calories")
	}

	return nil
}

func GetCaloriesFromMeals(meals *map[string][]models.Meal) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("failed to create gemini client %v", err)
		return
	}

	menuBytes, err := json.MarshalIndent(meals, "", " ")
	if err != nil {
		log.Printf("failed to marshal menu: %v", err)
		return
	}

	prompt := fmt.Sprintf(`
	You are a nutritional assistant. Here is a JSON menu structure from a university restaurant. 
	Return the exact same JSON structure, but add a "calories" field (string range representing kcal) to each meal object inside breakfast, lunch and dinner. An example would be "200-300", just numbers/range.

	JSON Menu:
	%s
	`, string(menuBytes))

	res, err := client.Models.GenerateContent(
		ctx,
		"gemini-3.1-flash-lite",
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	)
	if err != nil {
		log.Printf("procceding with the menu without calories: %v", err)
		return
	}

	response := []byte(res.Text())
	if err := validateCalorieResponse(menuBytes, response); err != nil {
		log.Printf("proceeding with the menu without calories: %v", err)
		return
	}

	var generated map[string][]models.Meal
	if err := json.Unmarshal(response, &generated); err != nil {
		log.Printf("error unmarshing json, continuing with the menu without calories")
		return
	}

	for mealType, generatedMeals := range generated {
		for i := range generatedMeals {
			(*meals)[mealType][i].Calories = generatedMeals[i].Calories
		}
	}

}
