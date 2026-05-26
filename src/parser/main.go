package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type sqsRecord struct {
	ImportID string  `json:"import_id"`
	RecordID string  `json:"record_id"`
	Row      int     `json:"row"`
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
}

type importHeader struct {
	ImportID    string `dynamodbav:"import_id"`
	FileHash    string `dynamodbav:"file_hash"`
	UserID      string `dynamodbav:"user_id"`
	Status      string `dynamodbav:"status"`
	Total       int    `dynamodbav:"total"`
	Succeeded   int    `dynamodbav:"succeeded"`
	Failed      int    `dynamodbav:"failed"`
	CreatedAt   string `dynamodbav:"created_at"`
	FinishedAt  string `dynamodbav:"finished_at,omitempty"`
	DuplicateOf string `dynamodbav:"duplicate_of,omitempty"`
}

var (
	s3Client     *s3.Client
	ddb          *dynamodb.Client
	sqsClient    *sqs.Client
	importsTable string
	queueURL     string
	logger       *slog.Logger
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("loading aws config: %v", err)
	}
	s3Client = s3.NewFromConfig(cfg)
	ddb = dynamodb.NewFromConfig(cfg)
	sqsClient = sqs.NewFromConfig(cfg)
	importsTable = os.Getenv("IMPORTS_TABLE")
	queueURL = os.Getenv("RECORDS_QUEUE_URL")
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func handler(ctx context.Context, event events.S3Event) error {
	for _, ev := range event.Records {
		if err := processObject(ctx, ev.S3.Bucket.Name, ev.S3.Object.Key); err != nil {
			// Returning the error would cause Lambda (and S3) to retry. We
			// don't want that: failures are persisted in the imports table
			// with the matching status. Log and move on.
			logger.Error("processing object failed",
				slog.String("bucket", ev.S3.Bucket.Name),
				slog.String("key", ev.S3.Object.Key),
				slog.String("error", err.Error()),
			)
		}
	}
	return nil
}

func processObject(ctx context.Context, bucket, key string) error {
	userID, importID, err := parseKey(key)
	if err != nil {
		return fmt.Errorf("parsing s3 key: %w", err)
	}

	logger.Info("processing object",
		slog.String("import_id", importID),
		slog.String("user_id", userID),
	)

	body, err := downloadObject(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("downloading object: %w", err)
	}

	fileHash := sha256Hex(body)

	existing, err := findOriginalImport(ctx, fileHash)
	if err != nil {
		return fmt.Errorf("checking idempotency: %w", err)
	}
	if existing != "" {
		logger.Info("duplicate detected",
			slog.String("import_id", importID),
			slog.String("duplicate_of", existing),
		)
		return saveHeader(ctx, importHeader{
			ImportID:    importID,
			FileHash:    fileHash,
			UserID:      userID,
			Status:      "done",
			CreatedAt:   nowISO(),
			FinishedAt:  nowISO(),
			DuplicateOf: existing,
		})
	}

	records, err := parseCSV(body)
	if err != nil {
		logger.Error("csv parse failed",
			slog.String("import_id", importID),
			slog.String("error", err.Error()),
		)
		return saveHeader(ctx, importHeader{
			ImportID:   importID,
			FileHash:   fileHash,
			UserID:     userID,
			Status:     "failed",
			CreatedAt:  nowISO(),
			FinishedAt: nowISO(),
		})
	}

	// Save header BEFORE publishing so workers can find it. With total=0
	// and no records yet published the import looks empty for a brief
	// window; that is acceptable for the MVP.
	if err := saveHeader(ctx, importHeader{
		ImportID:  importID,
		FileHash:  fileHash,
		UserID:    userID,
		Status:    "processing",
		Total:     len(records),
		CreatedAt: nowISO(),
	}); err != nil {
		return fmt.Errorf("saving header: %w", err)
	}

	if err := publishRecords(ctx, importID, records); err != nil {
		return fmt.Errorf("publishing records: %w", err)
	}

	logger.Info("dispatched records",
		slog.String("import_id", importID),
		slog.Int("count", len(records)),
	)
	return nil
}

// Expected key layout: uploads/<user_id>/<import_id>
func parseKey(key string) (userID, importID string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] != "uploads" || parts[1] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("unexpected key %q", key)
	}
	return parts[1], parts[2], nil
}

func downloadObject(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func parseCSV(body []byte) ([]sqsRecord, error) {
	r := csv.NewReader(strings.NewReader(string(body)))

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	if len(header) < 3 || header[0] != "sku" || header[1] != "name" || header[2] != "price" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}

	var out []sqsRecord
	rowNum := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row %d: %w", rowNum+1, err)
		}
		rowNum++

		var sku, name string
		var price float64
		if len(row) > 0 {
			sku = row[0]
		}
		if len(row) > 1 {
			name = row[1]
		}
		if len(row) > 2 {
			if p, perr := strconv.ParseFloat(row[2], 64); perr == nil {
				price = p
			}
		}

		out = append(out, sqsRecord{
			RecordID: fmt.Sprintf("row-%04d", rowNum),
			Row:      rowNum,
			SKU:      sku,
			Name:     name,
			Price:    price,
		})
	}
	return out, nil
}

// findOriginalImport returns the import_id of the first non-duplicate
// import already stored for this file_hash. Empty string means it is a
// new file.
func findOriginalImport(ctx context.Context, fileHash string) (string, error) {
	out, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(importsTable),
		IndexName:              aws.String("file_hash-index"),
		KeyConditionExpression: aws.String("file_hash = :h"),
		FilterExpression:       aws.String("attribute_not_exists(duplicate_of)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":h": &types.AttributeValueMemberS{Value: fileHash},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return "", err
	}
	if len(out.Items) == 0 {
		return "", nil
	}
	var hdr importHeader
	if err := attributevalue.UnmarshalMap(out.Items[0], &hdr); err != nil {
		return "", err
	}
	return hdr.ImportID, nil
}

func saveHeader(ctx context.Context, h importHeader) error {
	item, err := attributevalue.MarshalMap(h)
	if err != nil {
		return err
	}
	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(importsTable),
		Item:      item,
	})
	return err
}

func publishRecords(ctx context.Context, importID string, records []sqsRecord) error {
	const batchSize = 10

	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		entries := make([]sqstypes.SendMessageBatchRequestEntry, len(batch))
		for j, r := range batch {
			r.ImportID = importID
			body, err := json.Marshal(r)
			if err != nil {
				return err
			}
			entries[j] = sqstypes.SendMessageBatchRequestEntry{
				Id:          aws.String(strconv.Itoa(i + j)),
				MessageBody: aws.String(string(body)),
			}
		}

		out, err := sqsClient.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{
			QueueUrl: aws.String(queueURL),
			Entries:  entries,
		})
		if err != nil {
			return err
		}
		if len(out.Failed) > 0 {
			return fmt.Errorf("send batch had %d failures", len(out.Failed))
		}
	}
	return nil
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func main() {
	lambda.Start(handler)
}
