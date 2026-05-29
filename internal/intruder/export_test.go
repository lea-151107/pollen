package intruder

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleResults() []Result {
	return []Result{
		{Index: 0, Payload: "1", Status: 200, StatusText: "200 OK", Size: 42, DurationMs: 12, ContentType: "application/json"},
		{Index: 1, Payload: "2,3", Status: 404, StatusText: "404 Not Found", Size: 10, DurationMs: 4, ContentType: "text/plain"},
		{Index: 2, Payload: "boom", Status: 0, Error: "dial tcp: refused"},
	}
}

func TestCSV_HeaderAndRows(t *testing.T) {
	data, err := CSV(sampleResults())
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "index,payload,status,status_text,size_bytes,duration_ms,content_type,error\n") {
		t.Errorf("header mismatch: %q", s[:80])
	}
	// CSV must quote a payload containing a comma.
	if !strings.Contains(s, `"2,3"`) {
		t.Errorf("comma in payload not quoted:\n%s", s)
	}
	if !strings.Contains(s, "dial tcp: refused") {
		t.Errorf("error column missing")
	}
}

func TestJSON_EmptySliceMarshalsAsEmptyArray(t *testing.T) {
	data, err := JSON(nil)
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("want '[]', got %q", string(data))
	}
}

func TestJSON_RoundTrip(t *testing.T) {
	data, err := JSON(sampleResults())
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var got []Result
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 3 || got[1].Payload != "2,3" || got[2].Error == "" {
		t.Errorf("round trip wrong: %+v", got)
	}
}
