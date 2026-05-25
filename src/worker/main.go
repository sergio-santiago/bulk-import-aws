package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Returning an empty SQSEventResponse signals that all messages in the batch
// were processed successfully. Partial batch failures are reported via
// BatchItemFailures when the real logic lands.
func handler(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	return events.SQSEventResponse{}, nil
}

func main() {
	lambda.Start(handler)
}
