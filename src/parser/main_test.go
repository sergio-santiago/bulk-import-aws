package main

import (
	"os"
	"testing"
)

// These tests cover the pure half of the parser: the key layout, CSV decoding
// and the hash behind idempotency. Nothing here reaches AWS.
//
// The package does build its AWS clients in init(), which runs before any test
// does, so `make test` and CI pin AWS_REGION and switch off the instance
// metadata probe. Without that, a machine with no instance role spends a second
// per module waiting for a lookup that cannot succeed.

func TestParseKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		userID   string
		importID string
		wantErr  bool
	}{
		{"well formed", "uploads/user-1/import-9", "user-1", "import-9", false},
		{"wrong prefix", "downloads/user-1/import-9", "", "", true},
		{"no prefix", "user-1/import-9", "", "", true},
		{"missing import id", "uploads/user-1", "", "", true},
		{"empty user", "uploads//import-9", "", "", true},
		{"empty import", "uploads/user-1/", "", "", true},
		{"extra segment", "uploads/user-1/import-9/again", "", "", true},
		{"empty key", "", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userID, importID, err := parseKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseKey(%q) error = %v, wanted an error: %v", tc.key, err, tc.wantErr)
			}
			if userID != tc.userID || importID != tc.importID {
				t.Errorf("parseKey(%q) = %q, %q; want %q, %q",
					tc.key, userID, importID, tc.userID, tc.importID)
			}
		})
	}
}

// A malformed header is worth rejecting outright: every row below it is being
// read positionally, so the wrong header means every value lands in the wrong
// field rather than simply being absent.
func TestParseCSVRejectsUnexpectedHeader(t *testing.T) {
	tests := map[string]string{
		"different names":   "id,title,cost\nPROD-001,Laptop Pro,1299.99\n",
		"columns reordered": "name,sku,price\nLaptop Pro,PROD-001,1299.99\n",
		"one column short":  "sku,name\nPROD-001,Laptop Pro\n",
		"empty file":        "",
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCSV([]byte(body)); err == nil {
				t.Fatal("parseCSV accepted a file it should have rejected")
			}
		})
	}
}

// The boundary between the two halves of the pipeline, and the reason both of
// these tests exist next to each other.
//
// Structure is fatal here. A row with the wrong number of columns fails the
// whole file, because the reader is left with its default FieldsPerRecord and
// enforces the header's column count.
func TestParseCSVRejectsRaggedRows(t *testing.T) {
	body := "sku,name,price\nPROD-001,Laptop Pro,1299.99\nPROD-002,Wireless Mouse\n"
	if _, err := parseCSV([]byte(body)); err == nil {
		t.Fatal("parseCSV accepted a row with a missing column")
	}
}

// Content is not fatal. A price that will not parse becomes zero and travels
// on, and so do blank values, because rejecting them is the worker's job: one
// bad row should cost one row, not the whole import.
func TestParseCSVPassesBadContentThrough(t *testing.T) {
	body := "sku,name,price\n" +
		",Missing SKU,49.99\n" +
		"PROD-002,,29.50\n" +
		"PROD-003,Unparseable Price,not-a-number\n" +
		"PROD-004,Negative Price,-10.50\n"

	got, err := parseCSV([]byte(body))
	if err != nil {
		t.Fatalf("parseCSV returned an error on structurally valid rows: %v", err)
	}

	want := []sqsRecord{
		{RecordID: "row-0001", Row: 1, SKU: "", Name: "Missing SKU", Price: 49.99},
		{RecordID: "row-0002", Row: 2, SKU: "PROD-002", Name: "", Price: 29.50},
		{RecordID: "row-0003", Row: 3, SKU: "PROD-003", Name: "Unparseable Price", Price: 0},
		{RecordID: "row-0004", Row: 4, SKU: "PROD-004", Name: "Negative Price", Price: -10.50},
	}
	assertRecords(t, got, want)
}

// samples/valid.csv ships with the repository and the proof scripts upload it,
// so parsing it here keeps the fixture and the parser from drifting apart.
func TestParseCSVOnTheValidSample(t *testing.T) {
	got, err := parseCSV(readSample(t, "valid.csv"))
	if err != nil {
		t.Fatalf("parseCSV on samples/valid.csv: %v", err)
	}

	want := []sqsRecord{
		{RecordID: "row-0001", Row: 1, SKU: "PROD-001", Name: "Laptop Pro", Price: 1299.99},
		{RecordID: "row-0002", Row: 2, SKU: "PROD-002", Name: "Wireless Mouse", Price: 29.50},
		{RecordID: "row-0003", Row: 3, SKU: "PROD-003", Name: "Mechanical Keyboard", Price: 149.00},
		{RecordID: "row-0004", Row: 4, SKU: "PROD-004", Name: "USB-C Hub", Price: 79.99},
		{RecordID: "row-0005", Row: 5, SKU: "PROD-005", Name: "Monitor Stand", Price: 45.00},
	}
	assertRecords(t, got, want)
}

// samples/with-errors.csv is the other half of that pair. Every one of its rows
// is structurally sound, so all six reach the queue. Three of them are invalid
// on content and the worker is the one that says so: see TestRecordValidate.
func TestParseCSVOnTheSampleWithErrors(t *testing.T) {
	got, err := parseCSV(readSample(t, "with-errors.csv"))
	if err != nil {
		t.Fatalf("parseCSV on samples/with-errors.csv: %v", err)
	}

	want := []sqsRecord{
		{RecordID: "row-0001", Row: 1, SKU: "PROD-001", Name: "Laptop Pro", Price: 1299.99},
		{RecordID: "row-0002", Row: 2, SKU: "", Name: "Missing SKU", Price: 49.99},
		{RecordID: "row-0003", Row: 3, SKU: "PROD-003", Name: "Mechanical Keyboard", Price: 149.00},
		{RecordID: "row-0004", Row: 4, SKU: "PROD-004", Name: "", Price: 79.99},
		{RecordID: "row-0005", Row: 5, SKU: "PROD-005", Name: "Negative Price", Price: -10.50},
		{RecordID: "row-0006", Row: 6, SKU: "PROD-006", Name: "Valid Product", Price: 25.00},
	}
	assertRecords(t, got, want)
}

// A header on its own is a valid file with nothing in it. It must not be an
// error, or an empty upload would look like a broken one.
func TestParseCSVOnHeaderOnly(t *testing.T) {
	got, err := parseCSV([]byte("sku,name,price\n"))
	if err != nil {
		t.Fatalf("parseCSV on a header-only file: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d records from a header-only file, want 0", len(got))
	}
}

// Idempotency is decided by comparing this hash against the ones already
// stored, so its output has to stay stable across releases: a change here would
// silently re-import every file the system has ever seen.
func TestSHA256Hex(t *testing.T) {
	tests := map[string]string{
		"":                 "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"sku,name,price\n": "2cd1f2ddab23feef6d501664daaabbd91315c832f10657307a79534f7383bcb0",
	}

	for in, want := range tests {
		if got := sha256Hex([]byte(in)); got != want {
			t.Errorf("sha256Hex(%q) = %q, want %q", in, got, want)
		}
	}
}

// Two files that differ by a single byte have to hash apart, otherwise the
// second one would be filed away as a duplicate of the first.
func TestSHA256HexSeparatesNearIdenticalFiles(t *testing.T) {
	a := sha256Hex(readSample(t, "valid.csv"))
	b := sha256Hex(readSample(t, "with-errors.csv"))
	if a == b {
		t.Fatal("two different sample files produced the same hash")
	}
}

func readSample(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("../../samples/" + name)
	if err != nil {
		t.Fatalf("reading the sample: %v", err)
	}
	return body
}

func assertRecords(t *testing.T, got, want []sqsRecord) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
