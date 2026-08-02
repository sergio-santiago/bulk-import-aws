package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// jsonResponse is the only shape every endpoint here goes out through, so what
// it guarantees is the API's contract with the browser: a status, a JSON
// content type and a marshalled body.

func TestJSONResponse(t *testing.T) {
	body := importHeader{ImportID: "import-9", Status: "processing", Total: 5}

	resp, err := jsonResponse(http.StatusOK, body)
	if err != nil {
		t.Fatalf("jsonResponse: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Headers["content-type"]; got != "application/json" {
		t.Errorf("content-type = %q, want application/json", got)
	}

	var round importHeader
	if err := json.Unmarshal([]byte(resp.Body), &round); err != nil {
		t.Fatalf("the body did not survive a round trip: %v", err)
	}
	if round != body {
		t.Errorf("body = %+v, want %+v", round, body)
	}
}

// The struct tags decide what the browser sees, and two of them hide fields on
// purpose: the file hash is an implementation detail of deduplication and the
// user id is already known to whoever is asking.
func TestJSONResponseHidesInternalFields(t *testing.T) {
	resp, err := jsonResponse(http.StatusOK, importHeader{
		ImportID: "import-9",
		UserID:   "user-1",
		FileHash: "0f0f0f",
		Status:   "done",
	})
	if err != nil {
		t.Fatalf("jsonResponse: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("unmarshalling the body: %v", err)
	}
	for _, hidden := range []string{"user_id", "file_hash"} {
		if _, found := out[hidden]; found {
			t.Errorf("%q reached the response body", hidden)
		}
	}
	if out["import_id"] != "import-9" {
		t.Errorf("import_id = %v, want import-9", out["import_id"])
	}
}

// importDetail embeds importHeader without a name, which is what flattens the
// header's fields alongside failed_records instead of nesting them under a key.
// Naming that field, or tagging it, would silently reshape every detail
// response the front end reads.
func TestImportDetailFlattensTheHeader(t *testing.T) {
	resp, err := jsonResponse(http.StatusOK, importDetail{
		importHeader: importHeader{ImportID: "import-9", Status: "done", Total: 6, Failed: 3},
		FailedRecords: []failedRecord{
			{RecordID: "row-0002", Row: 2, Error: "sku is required"},
		},
	})
	if err != nil {
		t.Fatalf("jsonResponse: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("unmarshalling the body: %v", err)
	}
	for _, key := range []string{"import_id", "status", "total", "failed", "failed_records"} {
		if _, found := out[key]; !found {
			t.Errorf("%q is missing from the top level of the response", key)
		}
	}
	if _, nested := out["importHeader"]; nested {
		t.Error("the header was nested rather than flattened")
	}
}

// A body that cannot be marshalled degrades to a bare 500 and reports no error,
// because the caller is a Lambda handler and an error there is an invocation
// failure rather than an HTTP response.
func TestJSONResponseOnAnUnmarshalableBody(t *testing.T) {
	resp, err := jsonResponse(http.StatusOK, make(chan int))
	if err != nil {
		t.Fatalf("jsonResponse returned an error instead of a response: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
