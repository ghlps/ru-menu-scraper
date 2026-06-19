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

		if result != nil {
			out, marshalErr := json.MarshalIndent(result, "", "  ")
			if marshalErr != nil {
				log.Fatalf("failed to marshal result: %v", marshalErr)
			}
			fmt.Println(string(out))
		}

		if err != nil {
			log.Printf("Execution encountered error: %v", err)
			if result == nil {
				os.Exit(1)
			}
		} else {
			log.Println("Nothing to do, skipping (already sent or no menu found)")
		}
	} else {
		lambda.Start(svc.Handle)
	}
}
