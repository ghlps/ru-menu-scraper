package scraper

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ghlps/poc-go-scraper/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

func scrape(dateToScrape time.Time, restaurant models.Restaurant) (models.Menu, error) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36"),
	)

	c.SetRequestTimeout(15 * time.Second)

	c.OnRequest(func(r *colly.Request) {
		log.Printf("Visiting: %s", r.URL.String())
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("Everything connected with the %s", restaurant.Name)
	})

	c.OnError(func(r *colly.Response, err error) {
		if r != nil {
			log.Printf("Request URL: %s failed with response: %v\nStatus Code: %d\nError: %v", r.Request.URL, r, r.StatusCode, err)
		} else {
			log.Printf("Request failed with error: %v", err)
		}
	})

	return traverseDOM(dateToScrape, restaurant, c)
}

func traverseDOM(dateScraped time.Time, restaurant models.Restaurant, c *colly.Collector) (models.Menu, error) {
	formattedDate := dateScraped.Format("02/01/2006")

	state := &scrapeState{
		payload: models.Menu{
			Date:  formattedDate,
			Meals: make(map[string][]models.Meal),
		},
	}

	state.parseMenuForDate(c, formattedDate)

	c.OnScraped(func(r *colly.Response) {
		state.saveMeals()
		log.Println("Scraping completed")
	})

	if err := c.Visit(restaurant.Url); err != nil {
		return models.Menu{}, fmt.Errorf("visit page: %w", err)
	}

	if !hasAnyMeals(state.payload.Meals) {
		return models.Menu{}, nil
	} else {
		return state.payload, nil
	}
}

func hasAnyMeals(meals map[string][]models.Meal) bool {
	for _, items := range meals {
		if len(items) > 0 {
			return true
		}
	}
	return false
}

func (s *scrapeState) parseMenuForDate(c *colly.Collector, formattedDate string) {
	log.Printf("Trying to parse and transverse the menu using the date %s", formattedDate)
	c.OnHTML("div", func(e *colly.HTMLElement) {
		foundDate := false
		e.DOM.Children().Each(func(_ int, sel *goquery.Selection) {
			strongText := strings.TrimSpace(sel.Find("strong").Text())

			if foundDate && strings.Contains(strongText, "/") {
				foundDate = false
				return
			}
			if strings.Contains(strongText, formattedDate) {
				foundDate = true
				return
			}

			if foundDate && sel.Is("figure.wp-block-table") {
				sel.Find("tr").Each(func(_ int, row *goquery.Selection) {
					row.Find("td").Each(func(_ int, cell *goquery.Selection) {
						s.processHTMLCell(cell)
					})
				})
				foundDate = false
			}
		})
	})
}

func (s *scrapeState) processHTMLCell(cell *goquery.Selection) {
	htmlContent := strings.ToUpper(strings.TrimSpace(cell.Text()))

	if isMealType(htmlContent) {
		s.saveMeals()
		s.currentMealType = parseMealType(htmlContent)
		return
	}

	cellHTML, err := cell.Html()
	if err != nil {
		log.Printf("Error getting cell HTML: %v", err)
		return
	}

	for _, part := range strings.Split(cellHTML, "\n") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		meal, err := parseMeal(part)
		if err != nil || meal.Name == "" {
			continue
		}

		log.Printf("Adding meal: %s | icons: %v", meal.Name, meal.Icons)
		s.mealOptions = append(s.mealOptions, meal)
	}
}

type scrapeState struct {
	dateFound       bool
	tableFound      bool
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
