package db

import (
	"context"
	"fmt"
	"os"

	appconfig "github.com/ghlps/poc-go-scraper/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ghlps/poc-go-scraper/internal/models"
)

const tableName = "scraper-menu-executions"

type Store struct {
	client *dynamodb.Client
}

func NewStore(ctx context.Context, cfgApp appconfig.Config) (*Store, error) {
	var client *dynamodb.Client

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion("us-east-1"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
		)

		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}

		client = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfgApp.DynamoURL)
		})
	} else {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load aws config: %w", err)
		}
		client = dynamodb.NewFromConfig(cfg)
	}

	return &Store{client: client}, nil
}

func (s *Store) Save(ctx context.Context, data models.ScraperExecution) error {
	item, err := attributevalue.MarshalMap(data)
	if err != nil {
		return fmt.Errorf("marshal response data: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put item: %w", err)
	}

	return nil
}

func (s *Store) HasFailedExecutionForDate(ctx context.Context, date string) (bool, error) {
	out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("begins_with(created_at, :date) AND #st = :status"),
		ExpressionAttributeNames: map[string]string{
			"#st": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":date":   &types.AttributeValueMemberS{Value: date},
			":status": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", models.ExecutionStatusFailed)},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("scan failed executions for date %s: %w", date, err)
	}

	return len(out.Items) > 0, nil
}

func (s *Store) GetLatestByDay(ctx context.Context, date string, ruCode string) (*models.ScraperExecution, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("restaurant-code-date-index"),
		KeyConditionExpression: aws.String("restaurant_code = :ru AND menu_date = :date"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":date": &types.AttributeValueMemberS{Value: date},
			":ru":   &types.AttributeValueMemberS{Value: ruCode},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query latest execution for date %s: %w", date, err)
	}

	if len(out.Items) == 0 {
		return nil, nil
	}

	var executions []models.ScraperExecution
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &executions); err != nil {
		return nil, fmt.Errorf("unmarshal executions: %w", err)
	}

	latest := executions[0]
	for _, e := range executions[1:] {
		if e.CreatedAt.After(latest.CreatedAt) {
			latest = e
		}
	}

	return &latest, nil
}
