package importer

import (
	"os"
	"path/filepath"
	"testing"
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
