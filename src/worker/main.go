package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type record struct {
	ImportID string  `json:"import_id"`
	RecordID string  `json:"record_id"`
	Row      int     `json:"row"`
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
}

func (r record) validate() error {
	if strings.TrimSpace(r.SKU) == "" {
		return errors.New("sku is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	if r.Price <= 0 {
		return errors.New("price must be greater than zero")
	}
	return nil
}

var (
	ddb                *dynamodb.Client
	importsTable       string
	importRecordsTable string

	// Ready before main runs, so it is never nil: the handlers log on paths
	// the tests exercise without any AWS client in play.
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

// setup builds the AWS clients and reads the configuration. It is called from
// main rather than from init so that the test binary never builds a client it
// has no use for. Lambda still pays for it during the init phase, before the
// first invocation, which is where the free CPU burst is.
func setup() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("loading aws config: %v", err)
	}
	ddb = dynamodb.NewFromConfig(cfg)
	importsTable = os.Getenv("IMPORTS_TABLE")
	importRecordsTable = os.Getenv("IMPORT_RECORDS_TABLE")
}

func handler(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	var batchFailures []events.SQSBatchItemFailure

	for _, msg := range event.Records {
		if err := processMessage(ctx, msg); err != nil {
			logger.ErrorContext(ctx, "processing failed, sqs will retry",
				slog.String("message_id", msg.MessageId),
				slog.String("error", err.Error()),
			)
			batchFailures = append(batchFailures, events.SQSBatchItemFailure{
				ItemIdentifier: msg.MessageId,
			})
		}
	}

	return events.SQSEventResponse{BatchItemFailures: batchFailures}, nil
}

// processMessage returns a non-nil error only for transient failures that
// should be retried (eg. DynamoDB throttling). Validation errors are
// permanent and are persisted as failed records instead of bubbling up.
func processMessage(ctx context.Context, msg events.SQSMessage) error {
	var r record
	if err := json.Unmarshal([]byte(msg.Body), &r); err != nil {
		logger.ErrorContext(ctx, "malformed sqs body, discarding",
			slog.String("message_id", msg.MessageId),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if err := r.validate(); err != nil {
		if storeErr := saveFailure(ctx, r, err.Error()); storeErr != nil {
			return fmt.Errorf("saving failed record: %w", storeErr)
		}
		return incrementCounter(ctx, r.ImportID, "failed")
	}

	if err := saveSuccess(ctx, r); err != nil {
		return fmt.Errorf("saving successful record: %w", err)
	}
	return incrementCounter(ctx, r.ImportID, "succeeded")
}

func saveSuccess(ctx context.Context, r record) error {
	item, err := attributevalue.MarshalMap(struct {
		ImportID string  `dynamodbav:"import_id"`
		RecordID string  `dynamodbav:"record_id"`
		Status   string  `dynamodbav:"status"`
		SKU      string  `dynamodbav:"sku"`
		Name     string  `dynamodbav:"name"`
		Price    float64 `dynamodbav:"price"`
		Row      int     `dynamodbav:"row"`
	}{
		ImportID: r.ImportID,
		RecordID: r.RecordID,
		Status:   "success",
		SKU:      r.SKU,
		Name:     r.Name,
		Price:    r.Price,
		Row:      r.Row,
	})
	if err != nil {
		return err
	}
	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(importRecordsTable),
		Item:      item,
	})
	return err
}

func saveFailure(ctx context.Context, r record, reason string) error {
	item, err := attributevalue.MarshalMap(struct {
		ImportID string `dynamodbav:"import_id"`
		RecordID string `dynamodbav:"record_id"`
		Status   string `dynamodbav:"status"`
		Error    string `dynamodbav:"error"`
		Row      int    `dynamodbav:"row"`
	}{
		ImportID: r.ImportID,
		RecordID: r.RecordID,
		Status:   "failed",
		Error:    reason,
		Row:      r.Row,
	})
	if err != nil {
		return err
	}
	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(importRecordsTable),
		Item:      item,
	})
	return err
}

// incrementCounter atomically bumps `succeeded` or `failed` on the imports
// header. When the sum reaches `total`, this worker tries to mark the
// import as done; the ConditionExpression guarantees only one worker wins.
func incrementCounter(ctx context.Context, importID, counter string) error {
	if counter != "succeeded" && counter != "failed" {
		return fmt.Errorf("invalid counter %q", counter)
	}

	out, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(importsTable),
		Key: map[string]types.AttributeValue{
			"import_id": &types.AttributeValueMemberS{Value: importID},
		},
		UpdateExpression: aws.String("ADD #c :one"),
		ExpressionAttributeNames: map[string]string{
			"#c": counter,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return err
	}

	var counters struct {
		Total     int `dynamodbav:"total"`
		Succeeded int `dynamodbav:"succeeded"`
		Failed    int `dynamodbav:"failed"`
	}
	if err := attributevalue.UnmarshalMap(out.Attributes, &counters); err != nil {
		return fmt.Errorf("unmarshaling counters: %w", err)
	}

	if counters.Total > 0 && counters.Succeeded+counters.Failed >= counters.Total {
		return markDone(ctx, importID)
	}
	return nil
}

func markDone(ctx context.Context, importID string) error {
	_, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(importsTable),
		Key: map[string]types.AttributeValue{
			"import_id": &types.AttributeValueMemberS{Value: importID},
		},
		UpdateExpression:    aws.String("SET #s = :done, finished_at = :now"),
		ConditionExpression: aws.String("#s = :processing"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":done":       &types.AttributeValueMemberS{Value: "done"},
			":processing": &types.AttributeValueMemberS{Value: "processing"},
			":now":        &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})

	// Another worker already marked the import as done. Not an error.
	var ccfe *types.ConditionalCheckFailedException
	if errors.As(err, &ccfe) {
		return nil
	}
	return err
}

func main() {
	setup()
	lambda.Start(handler)
}
