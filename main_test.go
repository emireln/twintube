package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The error text echoes the input URL. Quotes and backslashes in it used to produce
// invalid JSON, which the browser could not parse.
func TestVideoInfoErrorIsValidJSON(t *testing.T) {
	query := url.Values{"v": {`https://example.com/a"b\c`}}
	req := httptest.NewRequest(http.MethodGet, "/api/video/info?"+query.Encode(), nil)
	rec := httptest.NewRecorder()

	handleVideoInfo(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v (%s)", err, rec.Body.String())
	}
	if body["error"] == "" {
		t.Fatalf("expected an error message, got %s", rec.Body.String())
	}
}
