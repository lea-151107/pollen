package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/exporter"
	"github.com/lea-151107/pollen/internal/history"
)

const sampleOpenAPIJSON = `{
  "openapi": "3.0.0",
  "info": {"title": "Test API"},
  "servers": [{"url": "https://api.example.com"}],
  "paths": {
    "/users": {
      "get": {"summary": "List users", "operationId": "listUsers"},
      "post": {"summary": "Create user"}
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "parameters": [{"name": "fields", "in": "query", "required": true}]
      }
    }
  }
}`

const sampleOpenAPIYAML = `
openapi: "3.0.0"
info:
  title: Test API
servers:
  - url: https://api.example.com
paths:
  /items:
    get:
      summary: List items
`

const samplePostman = `{
  "info": {"name": "My Collection", "_postman_id": "abc"},
  "item": [
    {
      "name": "Login",
      "request": {
        "method": "POST",
        "url": {"raw": "https://api.example.com/login"},
        "header": [{"key": "Content-Type", "value": "application/json"}],
        "body": {"mode": "raw", "raw": "{\"user\":\"alice\"}"}
      }
    },
    {
      "name": "Users folder",
      "item": [
        {
          "name": "Get user",
          "request": {
            "method": "GET",
            "url": {"raw": "https://api.example.com/users/1"},
            "header": []
          }
        }
      ]
    }
  ]
}`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportOpenAPIJSON(t *testing.T) {
	path := writeTemp(t, "openapi.json", sampleOpenAPIJSON)
	entries, err := Import(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	// Paths are sorted, so /users GET comes first.
	if entries[0].Request.Method != "GET" || entries[0].Name != "List users" {
		t.Errorf("entry[0]: got %q / %q", entries[0].Name, entries[0].Request.Method)
	}
	if entries[0].Request.URL != "https://api.example.com/users" {
		t.Errorf("entry[0] URL: got %q", entries[0].Request.URL)
	}
	// Required query param appended.
	if entries[2].Request.URL != "https://api.example.com/users/{id}?fields=" {
		t.Errorf("entry[2] URL with query param: got %q", entries[2].Request.URL)
	}
	// IDs must be non-empty.
	for i, e := range entries {
		if e.ID == "" {
			t.Errorf("entry[%d] has empty ID", i)
		}
	}
}

func TestImportOpenAPIYAML(t *testing.T) {
	path := writeTemp(t, "openapi.yaml", sampleOpenAPIYAML)
	entries, err := Import(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "List items" {
		t.Errorf("name: got %q", entries[0].Name)
	}
}

func TestImportPostman(t *testing.T) {
	path := writeTemp(t, "collection.json", samplePostman)
	entries, err := Import(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "Login" || entries[0].Request.Method != "POST" {
		t.Errorf("entry[0]: %+v", entries[0])
	}
	if entries[0].Request.Body != `{"user":"alice"}` {
		t.Errorf("body: got %q", entries[0].Request.Body)
	}
	if entries[1].Name != "Get user" {
		t.Errorf("nested item: got %q", entries[1].Name)
	}
}

func TestImportPostman_UrlencodedBody(t *testing.T) {
	const collection = `{
	  "info": {"name": "C"},
	  "item": [{
	    "name": "Login",
	    "request": {
	      "method": "POST",
	      "url": {"raw": "https://api.example.com/login"},
	      "body": {
	        "mode": "urlencoded",
	        "urlencoded": [
	          {"key": "user", "value": "alice"},
	          {"key": "pass", "value": "hunter2"}
	        ]
	      }
	    }
	  }]
	}`
	path := writeTemp(t, "collection.json", collection)
	entries, err := Import(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if got, want := string(entries[0].Request.BodyType), "form"; got != want {
		t.Errorf("BodyType: got %q want %q", got, want)
	}
	if got, want := entries[0].Request.Body, "user=alice\npass=hunter2"; got != want {
		t.Errorf("Body: got %q want %q", got, want)
	}
}

func TestImportPostman_GraphQLBody(t *testing.T) {
	const collection = `{
	  "info": {"name": "C"},
	  "item": [{
	    "name": "Users",
	    "request": {
	      "method": "POST",
	      "url": {"raw": "https://api.example.com/graphql"},
	      "body": {
	        "mode": "graphql",
	        "graphql": {
	          "query": "query { users { id } }",
	          "variables": "{\"limit\": 10}"
	        }
	      }
	    }
	  }]
	}`
	path := writeTemp(t, "collection.json", collection)
	entries, err := Import(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if got, want := string(entries[0].Request.BodyType), "graphql"; got != want {
		t.Errorf("BodyType: got %q want %q", got, want)
	}
	if got, want := entries[0].Request.Body, "query { users { id } }"; got != want {
		t.Errorf("Body (query): got %q want %q", got, want)
	}
	if got, want := entries[0].Request.GraphQLVariables, `{"limit": 10}`; got != want {
		t.Errorf("GraphQLVariables: got %q want %q", got, want)
	}
}

// TestExportImportRoundtripFormBody is the regression test for the bug where
// Pollen exported form bodies as {"mode": "urlencoded", "raw": "..."} while
// the importer only accepted mode=raw, so a form body survived export but
// vanished on re-import.
func TestExportImportRoundtripFormBody(t *testing.T) {
	original := []collections.Entry{{
		ID:   "src-1",
		Name: "Login",
		Request: history.Request{
			Method:   "POST",
			URL:      "https://api.example.com/login",
			BodyType: history.BodyForm,
			Body:     "user=alice\npass=hunter2",
		},
	}}
	data, err := exporter.ExportPostman(original, "c")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	path := writeTemp(t, "rt.json", string(data))

	restored, err := Import(path)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(restored) != 1 {
		t.Fatalf("want 1 entry, got %d", len(restored))
	}
	got := restored[0].Request
	if got.Method != "POST" || got.URL != original[0].Request.URL {
		t.Errorf("method/url mismatch: %+v", got)
	}
	if got.BodyType != history.BodyForm {
		t.Errorf("BodyType: got %q want form", got.BodyType)
	}
	if got.Body != original[0].Request.Body {
		t.Errorf("Body roundtrip lost data: got %q want %q", got.Body, original[0].Request.Body)
	}
}

func TestImportUnrecognised(t *testing.T) {
	path := writeTemp(t, "random.json", `{"foo": "bar"}`)
	_, err := Import(path)
	if err == nil {
		t.Fatal("expected error for unrecognised format")
	}
}

func TestImportMissingFile(t *testing.T) {
	_, err := Import("/nonexistent/path/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
