package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ghlps/poc-go-scraper/internal/config"
	"github.com/ghlps/poc-go-scraper/internal/scraper"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	svc, err := scraper.New(ctx, &cfg)
	if err != nil {
		log.Fatalf("failed to initialize scraper: %v", err)
	}

	if cfg.IsDev {
		data, err := os.ReadFile("event.json")
		if err != nil {
			log.Fatalf("failed to read event.json: %v", err)
		}

		var event scraper.Event
		if err := json.Unmarshal(data, &event); err != nil {
			log.Fatalf("failed to parse event.json: %v", err)
		}

		log.Printf("Starting local scrape for: %s", event.RestaurantCode)
		result, err := svc.Handle(ctx, &event)
		if err != nil {
			log.Fatalf("Execution failed: %v", err)
		}

		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal result: %v", err)
		}

		log.Println("Local execution finished successfully")
		fmt.Println(string(out))
	} else {
		lambda.Start(svc.Handle)
	}
}
