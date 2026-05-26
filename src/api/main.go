package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type importHeader struct {
	ImportID    string `dynamodbav:"import_id" json:"import_id"`
	UserID      string `dynamodbav:"user_id" json:"-"`
	FileHash    string `dynamodbav:"file_hash" json:"-"`
	Status      string `dynamodbav:"status" json:"status"`
	Total       int    `dynamodbav:"total" json:"total"`
	Succeeded   int    `dynamodbav:"succeeded" json:"succeeded"`
	Failed      int    `dynamodbav:"failed" json:"failed"`
	CreatedAt   string `dynamodbav:"created_at" json:"created_at"`
	FinishedAt  string `dynamodbav:"finished_at,omitempty" json:"finished_at,omitempty"`
	DuplicateOf string `dynamodbav:"duplicate_of,omitempty" json:"duplicate_of,omitempty"`
}

type failedRecord struct {
	RecordID string `dynamodbav:"record_id" json:"record_id"`
	Row      int    `dynamodbav:"row" json:"row"`
	SKU      string `dynamodbav:"sku,omitempty" json:"sku,omitempty"`
	Error    string `dynamodbav:"error" json:"error"`
}

type importDetail struct {
	importHeader
	FailedRecords []failedRecord `json:"failed_records"`
}

const presignTTL = 15 * time.Minute

var (
	s3Presigner        *s3.PresignClient
	ddb                *dynamodb.Client
	importsTable       string
	importRecordsTable string
	uploadsBucket      string
	logger             *slog.Logger
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("loading aws config: %v", err)
	}
	s3Presigner = s3.NewPresignClient(s3.NewFromConfig(cfg))
	ddb = dynamodb.NewFromConfig(cfg)
	importsTable = os.Getenv("IMPORTS_TABLE")
	importRecordsTable = os.Getenv("IMPORT_RECORDS_TABLE")
	uploadsBucket = os.Getenv("UPLOADS_BUCKET")
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	userID := req.RequestContext.Authorizer.JWT.Claims["sub"]
	if userID == "" {
		return jsonResponse(401, map[string]string{"error": "missing user"})
	}

	switch req.RouteKey {
	case "POST /imports":
		return createImport(ctx, userID)
	case "GET /imports":
		return listImports(ctx, userID)
	case "GET /imports/{id}":
		return getImport(ctx, userID, req.PathParameters["id"])
	default:
		return jsonResponse(404, map[string]string{"error": "not found"})
	}
}

func createImport(ctx context.Context, userID string) (events.APIGatewayV2HTTPResponse, error) {
	importID := uuid.NewString()
	key := fmt.Sprintf("uploads/%s/%s", userID, importID)

	req, err := s3Presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(uploadsBucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(presignTTL))
	if err != nil {
		logger.Error("presign failed", slog.String("error", err.Error()))
		return jsonResponse(500, map[string]string{"error": "presign failed"})
	}

	return jsonResponse(200, map[string]any{
		"import_id":  importID,
		"upload_url": req.URL,
		"expires_in": int(presignTTL.Seconds()),
	})
}

func getImport(ctx context.Context, userID, importID string) (events.APIGatewayV2HTTPResponse, error) {
	if importID == "" {
		return jsonResponse(400, map[string]string{"error": "missing id"})
	}

	out, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(importsTable),
		Key: map[string]types.AttributeValue{
			"import_id": &types.AttributeValueMemberS{Value: importID},
		},
	})
	if err != nil {
		logger.Error("get import failed", slog.String("error", err.Error()))
		return jsonResponse(500, map[string]string{"error": "internal"})
	}
	if out.Item == nil {
		return jsonResponse(404, map[string]string{"error": "not found"})
	}

	var hdr importHeader
	if err := attributevalue.UnmarshalMap(out.Item, &hdr); err != nil {
		logger.Error("unmarshal import failed", slog.String("error", err.Error()))
		return jsonResponse(500, map[string]string{"error": "internal"})
	}

	// Authorization: hide imports owned by other users behind 404 so we
	// don't leak existence.
	if hdr.UserID != userID {
		return jsonResponse(404, map[string]string{"error": "not found"})
	}

	failed, err := queryFailedRecords(ctx, importID)
	if err != nil {
		logger.Error("query failed records failed", slog.String("error", err.Error()))
		return jsonResponse(500, map[string]string{"error": "internal"})
	}

	return jsonResponse(200, importDetail{
		importHeader:  hdr,
		FailedRecords: failed,
	})
}

func listImports(ctx context.Context, userID string) (events.APIGatewayV2HTTPResponse, error) {
	// Scan with filter is fine for MVP. A user_id GSI would scale better
	// but adds infra weight not justified yet.
	out, err := ddb.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(importsTable),
		FilterExpression: aws.String("user_id = :u"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":u": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		logger.Error("scan imports failed", slog.String("error", err.Error()))
		return jsonResponse(500, map[string]string{"error": "internal"})
	}

	imports := make([]importHeader, 0, len(out.Items))
	for _, it := range out.Items {
		var hdr importHeader
		if err := attributevalue.UnmarshalMap(it, &hdr); err == nil {
			imports = append(imports, hdr)
		}
	}

	return jsonResponse(200, map[string]any{"imports": imports})
}

func queryFailedRecords(ctx context.Context, importID string) ([]failedRecord, error) {
	out, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(importRecordsTable),
		KeyConditionExpression: aws.String("import_id = :id"),
		FilterExpression:       aws.String("#s = :failed"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id":     &types.AttributeValueMemberS{Value: importID},
			":failed": &types.AttributeValueMemberS{Value: "failed"},
		},
	})
	if err != nil {
		return nil, err
	}
	records := make([]failedRecord, 0, len(out.Items))
	for _, it := range out.Items {
		var r failedRecord
		if err := attributevalue.UnmarshalMap(it, &r); err == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

func jsonResponse(status int, body any) (events.APIGatewayV2HTTPResponse, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{StatusCode: 500}, nil
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       string(b),
	}, nil
}

func main() {
	lambda.Start(handler)
}
