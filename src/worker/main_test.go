package main

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// The worker is the half of the pipeline that judges content. The parser hands
// over every row whose shape is right, blank fields and all, so these rules are
// the only thing standing between a bad row and the records table.
//
// The cases named after samples/with-errors.csv are the ones that file exists
// to produce. Its counterpart in the parser, TestParseCSVOnTheSampleWithErrors,
// asserts that all six of its rows survive parsing.
func TestRecordValidate(t *testing.T) {
	tests := []struct {
		name   string
		record record
		want   string
	}{
		{"complete", record{SKU: "PROD-001", Name: "Laptop Pro", Price: 1299.99}, ""},
		{"sample row 2, no sku", record{Name: "Missing SKU", Price: 49.99}, "sku is required"},
		{"sample row 4, no name", record{SKU: "PROD-004", Price: 79.99}, "name is required"},
		{"sample row 5, negative price", record{SKU: "PROD-005", Name: "Negative Price", Price: -10.50}, "price must be greater than zero"},
		{"whitespace sku", record{SKU: "  \t ", Name: "Laptop Pro", Price: 10}, "sku is required"},
		{"whitespace name", record{SKU: "PROD-001", Name: "\n", Price: 10}, "name is required"},
		{"zero price", record{SKU: "PROD-001", Name: "Laptop Pro", Price: 0}, "price must be greater than zero"},
		{"the smallest price that passes", record{SKU: "PROD-001", Name: "Laptop Pro", Price: 0.01}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.record.validate()
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("validate() rejected a valid record: %v", err)
			case tc.want != "" && err == nil:
				t.Fatalf("validate() accepted a record it should have rejected, wanted %q", tc.want)
			case tc.want != "" && err.Error() != tc.want:
				t.Errorf("validate() = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

// A record that fails the first rule must not be reported against the second.
// The order matters because the reason is written to the records table and read
// back by the API, so it is what the user is eventually shown.
func TestRecordValidateReportsTheFirstFailure(t *testing.T) {
	err := record{SKU: "", Name: "", Price: 0}.validate()
	if err == nil {
		t.Fatal("validate() accepted an entirely empty record")
	}
	if err.Error() != "sku is required" {
		t.Errorf("validate() = %q, want the first rule to be the one reported", err.Error())
	}
}

// A message that is not valid JSON can never become valid, so retrying it would
// occupy the queue until the redrive policy gave up. It is dropped instead, and
// dropping it means returning nil: any error here would send it round again.
func TestProcessMessageDiscardsMalformedBody(t *testing.T) {
	msg := events.SQSMessage{MessageId: "malformed-1", Body: "{not json"}
	if err := processMessage(context.Background(), msg); err != nil {
		t.Fatalf("processMessage returned %v, which would put the message back on the queue", err)
	}
}
