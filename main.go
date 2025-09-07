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
// and tracks them for TTL logging.
type RefreshLoggingProvider struct {
	Provider aws.CredentialsProvider

	mu        sync.Mutex
	lastCreds aws.Credentials
	first     bool
}

func (r *RefreshLoggingProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := r.Provider.Retrieve(ctx)
	if err != nil {
		log.Printf("[CREDENTIALS] failed to retrieve: %v", err)
		return creds, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.first ||
		creds.AccessKeyID != r.lastCreds.AccessKeyID ||
		creds.SecretAccessKey != r.lastCreds.SecretAccessKey ||
		creds.SessionToken != r.lastCreds.SessionToken {

		ttl := "N/A"
		if !creds.Expires.IsZero() {
			ttl = time.Until(creds.Expires).String()
		}

		log.Printf("[CREDENTIALS] REFRESHED: AccessKey=%s, ExpiresIn=%s, SessionTokenPresent=%v",
			creds.AccessKeyID, ttl, creds.SessionToken != "")

		r.lastCreds = creds
		r.first = false
	}

	return creds, nil
}

// StartTTLLogger periodically logs how long until creds expire
func (r *RefreshLoggingProvider) StartTTLLogger(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.mu.Lock()
				creds := r.lastCreds
				r.mu.Unlock()

				if creds.AccessKeyID == "" {
					continue // not retrieved yet
				}

				if creds.Expires.IsZero() {
					log.Printf("[CREDENTIALS] TTL check: permanent credentials, no expiration")
				} else {
					remaining := time.Until(creds.Expires)
					log.Printf("[CREDENTIALS] TTL check: %s remaining until expiration", remaining)
				}
			}
		}
	}()
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

	// Cache credentials for refresh support
	cachedProvider := aws.NewCredentialsCache(staticProvider)

	// Wrap with logging provider
	loggingProvider := &RefreshLoggingProvider{
		Provider: cachedProvider,
		first:    true,
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

	// Start periodic TTL logger
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loggingProvider.StartTTLLogger(ctx, 30*time.Second)

	fmt.Println(time.Now().Format("150405"))

	client := dynamodb.NewFromConfig(cfg)
	for {
		tableName := "MyTable" + time.Now().Format("150405")
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

		// Keep app alive a bit to see TTL logs
		time.Sleep(2 * time.Minute)
	}
}
