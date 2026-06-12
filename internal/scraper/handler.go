package scraper

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ghlps/poc-go-scraper/internal/config"
	"github.com/ghlps/poc-go-scraper/internal/db"
	"github.com/ghlps/poc-go-scraper/internal/models"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type Event struct {
	RestaurantCode string `json:"ruCode"`
	RunType        string `json:"runType"`
	DateOffset     int    `json:"dateOffset"`
}

type Scraper struct {
	store *db.Store
	cfg   *config.Config
}

func New(ctx context.Context, cfg *config.Config) (*Scraper, error) {
	store, err := db.NewStore(ctx, *cfg)
	if err != nil {
		return nil, err
	}
	return &Scraper{
		store: store,
		cfg:   cfg,
	}, nil
}

func (s *Scraper) Handle(ctx context.Context, event *Event) (*models.Menu, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system env vars")
	}

	restaurantCode, err := models.ParseRestaurantCode(event.RestaurantCode)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	restaurant := models.NewRestaurant(restaurantCode)
	timeToScrape := time.Now().AddDate(0, 0, event.DateOffset)
	formattedDateToScrape := timeToScrape.Format("02/01/2006")

	fmt.Println(restaurant)

	execution := models.ScraperExecution{
		ExecutionId:    uuid.New().String(),
		RestaurantCode: restaurantCode.String(),
		MenuDate:       formattedDateToScrape,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(72 * time.Hour).Unix(),
		Menu: &models.Menu{
			Restaurant: &restaurant,
			Meals:      make(map[string][]models.Meal),
		},
	}

	return s.run(ctx, &execution, formattedDateToScrape)
}

func (s *Scraper) run(ctx context.Context, execution *models.ScraperExecution, timeToScrape string) (*models.Menu, error) {
	menuData, err := scrape(timeToScrape, *execution.Menu.Restaurant)
	if err != nil {
		return nil, fmt.Errorf("scrape failed: %w", err)
	}

	menuData.Restaurant = execution.Menu.Restaurant

	if len(menuData.Meals) == 0 {
		if menuData.ImgMenu == nil {
			log.Printf("No meal was found for this specific date and no image captured")
			return nil, nil
		}
		log.Printf("No meals found, but image captured - returning result")
		return &menuData, nil
	}

	currentHash, err := db.HashMenu(&menuData)
	if err != nil {
		log.Printf("Warning: failed to hash menu: %v", err)
		return &menuData, fmt.Errorf("hashing failed: %w", err)
	}

	lastExecution, err := s.store.GetLatestByDay(ctx, timeToScrape, execution.Menu.Restaurant.Code.String())
	if err != nil {
		log.Printf("Warning: failed to query latest execution: %v - returning scraped data anyway", err)
		return &menuData, fmt.Errorf("db query failed: %w", err)
	}

	if lastExecution != nil {
		if lastExecution.MenuHash == currentHash {
			log.Printf("The menu didn't change %s, skipping save...", timeToScrape)
			return &menuData, nil
		}

		markChangedMeals(lastExecution.Menu, &menuData)
	}

	execution.Menu = &menuData
	execution.MenuHash = currentHash
	execution.Status = models.ExecutionStatusSuccess

	if err := s.store.Save(ctx, *execution); err != nil {
		log.Printf("Warning: failed to save to DB: %v - returning scraped data anyway", err)
		return &menuData, fmt.Errorf("db save failed: %w", err)
	}

	return &menuData, nil
}
