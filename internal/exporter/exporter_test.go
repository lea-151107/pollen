package exporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lea/pollen/internal/collections"
	"github.com/lea/pollen/internal/history"
)

func TestExportPostman_EmptyEntries(t *testing.T) {
	data, err := ExportPostman(nil, "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), `"name": "empty"`) {
		t.Errorf("expected collection name in output: %s", data)
	}
}

func TestExportPostman_RawBody(t *testing.T) {
	entries := []collections.Entry{
		{
			ID:   "1",
			Name: "Raw",
			Request: history.Request{
				Method:   "POST",
				URL:      "https://api.example.com/x",
				Body:     `{"a":1}`,
				BodyType: history.BodyRaw,
			},
		},
	}
	data, err := ExportPostman(entries, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got postmanExportCollection
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("export not valid JSON: %v", err)
	}
	if len(got.Item) != 1 {
		t.Fatalf("want 1 item, got %d", len(got.Item))
	}
	body := got.Item[0].Request.Body
	if body == nil {
		t.Fatal("body unexpectedly nil")
	}
	if body.Mode != "raw" || body.Raw != `{"a":1}` || len(body.URLEncoded) != 0 {
		t.Errorf("raw export: %+v", body)
	}
}

func TestExportPostman_FormBody(t *testing.T) {
	entries := []collections.Entry{
		{
			ID:   "1",
			Name: "Form",
			Request: history.Request{
				Method:   "POST",
				URL:      "https://api.example.com/x",
				Body:     "user=alice\npass=hunter2",
				BodyType: history.BodyForm,
			},
		},
	}
	data, err := ExportPostman(entries, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got postmanExportCollection
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("export not valid JSON: %v", err)
	}
	body := got.Item[0].Request.Body
	if body == nil {
		t.Fatal("body unexpectedly nil")
	}
	if body.Mode != "urlencoded" {
		t.Errorf("mode: got %q want urlencoded", body.Mode)
	}
	if body.Raw != "" {
		t.Errorf("raw should be empty for urlencoded mode, got %q", body.Raw)
	}
	if len(body.URLEncoded) != 2 {
		t.Fatalf("urlencoded len: %d", len(body.URLEncoded))
	}
	if body.URLEncoded[0] != (postmanExportFormParam{Key: "user", Value: "alice"}) {
		t.Errorf("entry 0: %+v", body.URLEncoded[0])
	}
	if body.URLEncoded[1] != (postmanExportFormParam{Key: "pass", Value: "hunter2"}) {
		t.Errorf("entry 1: %+v", body.URLEncoded[1])
	}
}

func TestExportPostman_FormBodyTrimAndSkipBlankLines(t *testing.T) {
	pairs := parseFormPairs("  a  =  1  \n\n  b=2\n   \nc")
	if len(pairs) != 2 {
		t.Fatalf("want 2 pairs (c has no '='), got %d: %+v", len(pairs), pairs)
	}
	if pairs[0] != (postmanExportFormParam{Key: "a", Value: "1"}) {
		t.Errorf("pair[0]: %+v", pairs[0])
	}
	if pairs[1] != (postmanExportFormParam{Key: "b", Value: "2"}) {
		t.Errorf("pair[1]: %+v", pairs[1])
	}
}

func TestExportPostman_Headers(t *testing.T) {
	entries := []collections.Entry{
		{
			Name: "h",
			Request: history.Request{
				Method: "GET",
				URL:    "https://api.example.com",
				Headers: []history.Header{
					{Key: "X-A", Value: "1"},
					{Key: "", Value: "skip-empty-key"},
					{Key: "X-B", Value: "2"},
				},
			},
		},
	}
	data, err := ExportPostman(entries, "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got postmanExportCollection
	_ = json.Unmarshal(data, &got)
	hdr := got.Item[0].Request.Header
	if len(hdr) != 2 {
		t.Fatalf("want 2 headers (empty key skipped), got %d", len(hdr))
	}
	if hdr[0]["key"] != "X-A" || hdr[1]["key"] != "X-B" {
		t.Errorf("header keys: %+v", hdr)
	}
}
