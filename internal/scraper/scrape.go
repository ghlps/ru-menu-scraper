package scraper

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ghlps/poc-go-scraper/internal/models"
	"github.com/gocolly/colly/v2"
)

const s3Bucket = "ru-menu-images"

func scrape(dateToScrape string, restaurant models.Restaurant) (models.Menu, error) {
	ctx := context.Background()

	s3Client, err := newS3Client(ctx)
	if err != nil {
		return models.Menu{}, fmt.Errorf("init S3 client: %w", err)
	}

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

	return traverseDOM(ctx, dateToScrape, restaurant, c, s3Client)
}

func traverseDOM(ctx context.Context, dateScraped string, restaurant models.Restaurant, c *colly.Collector, s3Client *s3.Client) (models.Menu, error) {
	state := &scrapeState{
		ctx:      ctx,
		s3Client: s3Client,
		payload: models.Menu{
			Date:  dateScraped,
			Meals: make(map[string][]models.Meal),
		},
	}

	state.parseMenuForDate(c, dateScraped, restaurant.Code.String())

	c.OnScraped(func(r *colly.Response) {
		state.saveMeals()
		log.Println("Scraping completed")
	})

	if err := c.Visit(restaurant.Url); err != nil {
		return models.Menu{}, fmt.Errorf("visit page: %w", err)
	}

	return state.payload, nil
}

func (s *scrapeState) parseMenuForDate(c *colly.Collector, formattedDate string, ruCode string) {
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

			if foundDate && sel.Is("div") {
				if img := sel.Find("img"); img.Length() > 0 {
					src, _ := img.Attr("src")
					log.Printf("Found image for date %s: %.30s", formattedDate, src)
					s.uploadAndStoreImage(src, formattedDate, ruCode)
					foundDate = false
				}
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

func (s *scrapeState) uploadImage(imageURL, dateScraped string, ruCode string) (string, error) {
	safeDate := strings.ReplaceAll(dateScraped, "/", "_")
	key := fmt.Sprintf("%s/%s_menu_%s.jpg", strings.ToLower(ruCode), safeDate, strings.ToLower(ruCode))

	if strings.HasPrefix(imageURL, "data:") {
		return s.uploadDataURI(key, imageURL)
	}

	resp, err := http.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return s.putS3Object(key, data, contentType)
}

func (s *scrapeState) uploadDataURI(key, dataURI string) (string, error) {
	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URI format")
	}
	if !strings.Contains(parts[0], "base64") {
		return "", fmt.Errorf("only base64 encoded data URIs are supported")
	}

	contentType := "image/jpeg"
	header := strings.TrimPrefix(parts[0], "data:")
	if ct := strings.Split(header, ";")[0]; ct != "" {
		contentType = ct
	}

	imageData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	return s.putS3Object(key, imageData, contentType)
}

func (s *scrapeState) putS3Object(key string, data []byte, contentType string) (string, error) {
	_, err := s.s3Client.PutObject(s.ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("upload to S3 (bucket=%s key=%s): %w", s3Bucket, key, err)
	}
	s3URL := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s3Bucket, key)
	log.Printf("Uploaded to S3: %s", s3URL)
	return s3URL, nil
}

func (s *scrapeState) uploadAndStoreImage(imageURL, dateScraped, ruCode string) {
	s3url, err := s.uploadImage(imageURL, dateScraped, ruCode)
	if err != nil {
		log.Printf("Error uploading image: %v", err)
		return
	}
	if s.payload.ImgMenu == nil {
		s.payload.ImgMenu = aws.String(s3url)
		log.Printf("Stored image URL in payload: %s", s3url)
	}
}

func newS3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return s3.NewFromConfig(cfg), nil
}
