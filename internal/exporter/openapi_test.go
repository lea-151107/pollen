package exporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
)

func decodeDoc(t *testing.T, data []byte) openAPIDoc {
	t.Helper()
	var doc openAPIDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode openapi: %v\n%s", err, data)
	}
	return doc
}

func TestExportOpenAPI_EmptyEntries(t *testing.T) {
	data, err := ExportOpenAPI(nil, "empty", OpenAPIJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc := decodeDoc(t, data)
	if doc.OpenAPI != "3.0.3" {
		t.Errorf("openapi version: want 3.0.3, got %q", doc.OpenAPI)
	}
	if doc.Info.Title != "empty" {
		t.Errorf("title: want %q, got %q", "empty", doc.Info.Title)
	}
	if doc.Info.Version != "1.0.0" {
		t.Errorf("version: want %q, got %q", "1.0.0", doc.Info.Version)
	}
	if len(doc.Servers) != 0 {
		t.Errorf("expected no servers block for empty entries, got %v", doc.Servers)
	}
	if len(doc.Paths) != 0 {
		t.Errorf("expected empty paths, got %v", doc.Paths)
	}
}

func TestExportOpenAPI_SingleEntryWithServer(t *testing.T) {
	entries := []collections.Entry{{
		ID:   "1",
		Name: "List users",
		Request: history.Request{
			Method: "GET",
			URL:    "https://api.example.com/users",
		},
	}}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "https://api.example.com" {
		t.Fatalf("server: want https://api.example.com, got %v", doc.Servers)
	}
	p, ok := doc.Paths["/users"]
	if !ok {
		t.Fatalf("expected /users path, got %v", doc.Paths)
	}
	if p.Get == nil || p.Get.Summary != "List users" {
		t.Errorf("GET /users summary mismatch: %+v", p.Get)
	}
}

func TestExportOpenAPI_GroupingByPath(t *testing.T) {
	entries := []collections.Entry{
		{ID: "1", Name: "List users", Request: history.Request{Method: "GET", URL: "https://api.example.com/users"}},
		{ID: "2", Name: "Create user", Request: history.Request{Method: "POST", URL: "https://api.example.com/users"}},
	}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	p, ok := doc.Paths["/users"]
	if !ok {
		t.Fatalf("expected /users path, got %v", doc.Paths)
	}
	if p.Get == nil {
		t.Errorf("expected GET operation under /users")
	}
	if p.Post == nil {
		t.Errorf("expected POST operation under /users")
	}
}

func TestExportOpenAPI_BodyModes(t *testing.T) {
	entries := []collections.Entry{
		{
			ID: "j", Name: "json", Request: history.Request{
				Method: "POST", URL: "https://api.example.com/a",
				Body: `{"x":1}`, BodyType: history.BodyJSON,
			},
		},
		{
			ID: "f", Name: "form", Request: history.Request{
				Method: "POST", URL: "https://api.example.com/b",
				Body: "k1=v1\nk2=v2", BodyType: history.BodyForm,
			},
		},
		{
			ID: "r", Name: "raw", Request: history.Request{
				Method: "POST", URL: "https://api.example.com/c",
				Body: "hello", BodyType: history.BodyRaw,
			},
		},
	}
	doc := decodeDoc(t, mustExport(t, entries, "api"))

	if rb := doc.Paths["/a"].Post.RequestBody; rb == nil || rb.Content["application/json"].Example != `{"x":1}` {
		t.Errorf("json body wrong: %+v", rb)
	}

	formMedia, ok := doc.Paths["/b"].Post.RequestBody.Content["application/x-www-form-urlencoded"]
	if !ok || formMedia.Schema == nil || formMedia.Schema.Type != "object" {
		t.Fatalf("form body wrong: %+v", doc.Paths["/b"].Post.RequestBody)
	}
	if formMedia.Schema.Properties["k1"].Example != "v1" || formMedia.Schema.Properties["k2"].Example != "v2" {
		t.Errorf("form body properties wrong: %+v", formMedia.Schema.Properties)
	}

	rawMedia, ok := doc.Paths["/c"].Post.RequestBody.Content["text/plain"]
	if !ok || rawMedia.Example != "hello" {
		t.Errorf("raw body wrong: %+v", doc.Paths["/c"].Post.RequestBody)
	}
}

func TestExportOpenAPI_RawBodyUsesContentTypeHeader(t *testing.T) {
	// A raw body with an explicit Content-Type header should land under that
	// media type, not text/plain. This keeps the spec consistent with what
	// httpx.buildBody would actually transmit.
	entries := []collections.Entry{{
		ID: "1", Name: "xml", Request: history.Request{
			Method:   "POST",
			URL:      "https://api.example.com/x",
			Headers:  []history.Header{{Key: "Content-Type", Value: "application/xml"}},
			Body:     "<doc/>",
			BodyType: history.BodyRaw,
		},
	}}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	if _, ok := doc.Paths["/x"].Post.RequestBody.Content["application/xml"]; !ok {
		t.Errorf("expected application/xml content type, got %+v", doc.Paths["/x"].Post.RequestBody.Content)
	}
}

func TestExportOpenAPI_HeadersAsParameters(t *testing.T) {
	entries := []collections.Entry{{
		ID: "1", Name: "get", Request: history.Request{
			Method: "GET", URL: "https://api.example.com/h",
			Headers: []history.Header{{Key: "X-Trace", Value: "abc"}},
		},
	}}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	params := doc.Paths["/h"].Get.Parameters
	if len(params) != 1 || params[0].Name != "X-Trace" || params[0].In != "header" || params[0].Example != "abc" {
		t.Errorf("header parameter wrong: %+v", params)
	}
}

func TestExportOpenAPI_QueryAsParameters(t *testing.T) {
	entries := []collections.Entry{{
		ID: "1", Name: "search", Request: history.Request{
			Method: "GET",
			URL:    "https://api.example.com/q?fields=name&page=2",
		},
	}}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	params := doc.Paths["/q"].Get.Parameters
	if len(params) != 2 {
		t.Fatalf("want 2 query params, got %d: %+v", len(params), params)
	}
	// queryParams sorts alphabetically, so "fields" precedes "page".
	if params[0].Name != "fields" || params[0].In != "query" || params[0].Example != "name" {
		t.Errorf("fields param wrong: %+v", params[0])
	}
	if params[1].Name != "page" || params[1].Example != "2" {
		t.Errorf("page param wrong: %+v", params[1])
	}
}

func TestExportOpenAPI_VariableTokensPreserved(t *testing.T) {
	// URLs that carry pollen's {{var}} tokens cannot be split into
	// scheme/host, so we must emit them as-is (no server, raw path) so the
	// operator's parameterisation survives.
	entries := []collections.Entry{{
		ID: "1", Name: "tmpl", Request: history.Request{
			Method: "GET", URL: "{{baseURL}}/users/{{id}}",
		},
	}}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	if len(doc.Servers) != 0 {
		t.Errorf("expected no servers block for tokenised URL, got %+v", doc.Servers)
	}
	if _, ok := doc.Paths["/{{baseURL}}/users/{{id}}"]; !ok {
		t.Errorf("expected raw URL preserved as path key, got %v", doc.Paths)
	}
}

func TestExportOpenAPI_OperationIDDedup(t *testing.T) {
	entries := []collections.Entry{
		{ID: "1", Name: "list", Request: history.Request{Method: "GET", URL: "https://api.example.com/a"}},
		{ID: "2", Name: "list", Request: history.Request{Method: "GET", URL: "https://api.example.com/b"}},
		{ID: "3", Name: "list", Request: history.Request{Method: "GET", URL: "https://api.example.com/c"}},
	}
	doc := decodeDoc(t, mustExport(t, entries, "api"))
	ids := map[string]bool{}
	for _, p := range []openAPIPath{doc.Paths["/a"], doc.Paths["/b"], doc.Paths["/c"]} {
		if p.Get == nil {
			t.Fatal("missing GET op")
		}
		if ids[p.Get.OperationID] {
			t.Errorf("duplicate operationId: %s", p.Get.OperationID)
		}
		ids[p.Get.OperationID] = true
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 unique operationIds, got %v", ids)
	}
}

func TestExportOpenAPI_YAMLFormat(t *testing.T) {
	entries := []collections.Entry{{
		ID: "1", Name: "List users", Request: history.Request{Method: "GET", URL: "https://api.example.com/users"},
	}}
	data, err := ExportOpenAPI(entries, "api", OpenAPIYAML)
	if err != nil {
		t.Fatalf("export yaml: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "openapi: 3.0.3") {
		t.Errorf("YAML output missing openapi version line:\n%s", s)
	}
	if !strings.Contains(s, "/users:") {
		t.Errorf("YAML output missing /users path:\n%s", s)
	}
	if !strings.Contains(s, "get:") {
		t.Errorf("YAML output missing get method:\n%s", s)
	}
}

func mustExport(t *testing.T, entries []collections.Entry, title string) []byte {
	t.Helper()
	data, err := ExportOpenAPI(entries, title, OpenAPIJSON)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return data
}
