package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// RefreshLoggingProvider logs only when credentials are refreshed
type RefreshLoggingProvider struct {
	Provider aws.CredentialsProvider

	mu         sync.Mutex
	lastCreds  aws.Credentials
	firstFetch bool
}

func (r *RefreshLoggingProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := r.Provider.Retrieve(ctx)
	if err != nil {
		log.Printf("[CREDENTIALS] failed to retrieve: %v", err)
		return creds, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Log only if credentials changed or first fetch
	if r.firstFetch || creds.AccessKeyID != r.lastCreds.AccessKeyID || creds.SecretAccessKey != r.lastCreds.SecretAccessKey || creds.SessionToken != r.lastCreds.SessionToken {
		ttl := "N/A"
		if !creds.Expires.IsZero() {
			ttl = time.Until(creds.Expires).String()
		}
		log.Printf("[CREDENTIALS] REFRESHED: AccessKey=%s, ExpiresIn=%s, SessionTokenPresent=%v",
			creds.AccessKeyID, ttl, creds.SessionToken != "")
		r.lastCreds = creds
		r.firstFetch = false
	}

	return creds, nil
}

func main() {
	endpoint := "http://localhost:4566"
	region := "us-west-2"

	staticProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			SessionToken:    "",
		},
	}

	// Wrap with AWS credentials cache for auto-refresh
	cachedProvider := aws.NewCredentialsCache(staticProvider)

	// Wrap cached provider with our logging wrapper
	loggingProvider := &RefreshLoggingProvider{
		Provider:   cachedProvider,
		firstFetch: true,
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithBaseEndpoint(endpoint),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(loggingProvider),
		),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	tableName := "MyTable"
	_, err = client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: &tableName,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("ID"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("ID"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}
	fmt.Println("Table created:", tableName)

	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: &tableName,
		Item: map[string]types.AttributeValue{
			"ID":   &types.AttributeValueMemberS{Value: "123"},
			"Name": &types.AttributeValueMemberS{Value: "LocalUser"},
		},
	})
	if err != nil {
		log.Fatalf("failed to put item: %v", err)
	}
	fmt.Println("Inserted item into table")

	resp, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"ID": &types.AttributeValueMemberS{Value: "123"},
		},
	})
	if err != nil {
		log.Fatalf("failed to get item: %v", err)
	}

	nameAttr := resp.Item["Name"].(*types.AttributeValueMemberS)
	fmt.Printf("Fetched item: ID=%s, Name=%s\n", "123", nameAttr.Value)
}
