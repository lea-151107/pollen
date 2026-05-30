// Package importer reads OpenAPI 3.x (JSON/YAML) and Postman Collection v2.1
// documents and converts them into pollen collection entries.
package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/google/uuid"
	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
)

// Import reads path, detects the format (OpenAPI 3.x or Postman v2.1), and
// returns one Entry per endpoint. Returns an error if the file can't be read
// or the format is unrecognised.
func Import(path string) ([]collections.Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return parseOpenAPIYAML(data)
	default:
		return parseJSON(data)
	}
}

// parseJSON detects whether data is a Postman collection or OpenAPI JSON doc.
func parseJSON(data []byte) ([]collections.Entry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, hasItem := raw["item"]; hasItem {
		return parsePostmanJSON(data)
	}
	if _, hasPaths := raw["paths"]; hasPaths {
		return parseOpenAPIJSON(data)
	}
	return nil, fmt.Errorf("unrecognised format: expected OpenAPI (paths) or Postman (item)")
}

// ── OpenAPI 3.x ─────────────────────────────────────────────────────────────

type openAPIDoc struct {
	OpenAPI string                              `json:"openapi" yaml:"openapi"`
	Info    struct{ Title string }              `json:"info"    yaml:"info"`
	Servers []struct{ URL string }              `json:"servers" yaml:"servers"`
	Paths   map[string]map[string]openAPIOpRaw  `json:"paths"   yaml:"paths"`
}

type openAPIOpRaw struct {
	OperationID string           `json:"operationId" yaml:"operationId"`
	Summary     string           `json:"summary"     yaml:"summary"`
	Parameters  []openAPIParam   `json:"parameters"  yaml:"parameters"`
}

type openAPIParam struct {
	Name     string `json:"name"     yaml:"name"`
	In       string `json:"in"       yaml:"in"`
	Required bool   `json:"required" yaml:"required"`
}

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true,
	"delete": true, "head": true, "options": true, "trace": true,
}

func openAPIToEntries(doc openAPIDoc) []collections.Entry {
	baseURL := ""
	if len(doc.Servers) > 0 {
		baseURL = strings.TrimRight(doc.Servers[0].URL, "/")
	}

	// Sort paths for deterministic output.
	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var entries []collections.Entry
	for _, path := range paths {
		ops := doc.Paths[path]
		// Sort methods too.
		methods := make([]string, 0, len(ops))
		for m := range ops {
			methods = append(methods, m)
		}
		sort.Strings(methods)

		for _, method := range methods {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			op := ops[method]
			name := op.Summary
			if name == "" {
				name = op.OperationID
			}
			if name == "" {
				name = strings.ToUpper(method) + " " + path
			}

			url := baseURL + path
			// Append required query params as placeholders.
			var qp []string
			for _, p := range op.Parameters {
				if strings.ToLower(p.In) == "query" && p.Required {
					qp = append(qp, p.Name+"=")
				}
			}
			if len(qp) > 0 {
				url += "?" + strings.Join(qp, "&")
			}

			entries = append(entries, collections.Entry{
				ID:   uuid.NewString(),
				Name: name,
				Request: history.Request{
					Method: strings.ToUpper(method),
					URL:    url,
				},
			})
		}
	}
	return entries
}

func parseOpenAPIJSON(data []byte) ([]collections.Entry, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("OpenAPI JSON parse error: %w", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3") {
		return nil, fmt.Errorf("only OpenAPI 3.x is supported (got %q)", doc.OpenAPI)
	}
	return openAPIToEntries(doc), nil
}

func parseOpenAPIYAML(data []byte) ([]collections.Entry, error) {
	var doc openAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("OpenAPI YAML parse error: %w", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3") {
		return nil, fmt.Errorf("only OpenAPI 3.x is supported (got %q)", doc.OpenAPI)
	}
	return openAPIToEntries(doc), nil
}

// ── Postman v2.1 ────────────────────────────────────────────────────────────

type postmanCollection struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name    string         `json:"name"`
	Request *postmanReq    `json:"request"`
	Item    []postmanItem  `json:"item"`
}

type postmanURL struct{ Raw string }

// UnmarshalJSON handles both the string form ("url": "https://...") and the
// object form ("url": {"raw": "https://..."}) that Postman v2.1 allows.
func (p *postmanURL) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Raw = s
		return nil
	}
	var obj struct{ Raw string }
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	p.Raw = obj.Raw
	return nil
}

type postmanReq struct {
	Method string     `json:"method"`
	URL    postmanURL `json:"url"`
	Header []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"header"`
	Body *struct {
		Mode       string `json:"mode"`
		Raw        string `json:"raw"`
		URLEncoded []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"urlencoded"`
		FormData []struct {
			Key         string `json:"key"`
			Type        string `json:"type"`
			Value       string `json:"value"`
			Src         string `json:"src"`
			ContentType string `json:"contentType"`
		} `json:"formdata"`
		GraphQL *struct {
			Query string `json:"query"`
			// Variables is raw JSON so we accept both the Postman
			// canonical string form ("variables": "{...}") and the
			// object form ("variables": {...}) emitted by some
			// third-party tools (Insomnia, hand-written collections).
			// Normalised to a JSON string when populating
			// history.Request.GraphQLVariables.
			Variables json.RawMessage `json:"variables"`
		} `json:"graphql"`
	} `json:"body"`
}

func parsePostmanJSON(data []byte) ([]collections.Entry, error) {
	var col postmanCollection
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, fmt.Errorf("Postman JSON parse error: %w", err)
	}
	var entries []collections.Entry
	walkPostman(col.Item, &entries)
	return entries, nil
}

func walkPostman(items []postmanItem, out *[]collections.Entry) {
	for _, item := range items {
		if item.Request != nil {
			*out = append(*out, postmanItemToEntry(item))
		}
		if len(item.Item) > 0 {
			walkPostman(item.Item, out)
		}
	}
}

// normalisePostmanGraphQLVariables takes a raw JSON value from a
// Postman collection's body.graphql.variables field and returns the
// JSON-string representation pollen stores in
// history.Request.GraphQLVariables. The Postman v2.1 spec says
// variables is a string, but real-world files (Insomnia exports,
// hand-edited collections) sometimes use the object form — handle
// both rather than failing the whole collection import.
func normalisePostmanGraphQLVariables(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	// String form: parsed JSON string, unwrap to the inner content.
	if s[0] == '"' {
		var inner string
		if err := json.Unmarshal(raw, &inner); err == nil {
			return inner
		}
		return ""
	}
	// Object / array / number / bool: keep as the raw JSON value.
	return s
}

func postmanItemToEntry(item postmanItem) collections.Entry {
	req := item.Request
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}

	var headers []history.Header
	for _, h := range req.Header {
		if h.Key != "" {
			headers = append(headers, history.Header{Key: h.Key, Value: h.Value})
		}
	}

	var body string
	var bodyType history.BodyType
	var graphqlVariables string
	if req.Body != nil {
		switch req.Body.Mode {
		case "raw":
			body = req.Body.Raw
			bodyType = history.BodyRaw
		case "urlencoded":
			var sb strings.Builder
			for i, p := range req.Body.URLEncoded {
				if i > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(p.Key)
				sb.WriteString("=")
				sb.WriteString(p.Value)
			}
			body = sb.String()
			bodyType = history.BodyForm
		case "formdata":
			// Convert Postman's array form back into pollen's
			// line-based multipart DSL.
			var sb strings.Builder
			first := true
			for _, p := range req.Body.FormData {
				if p.Key == "" {
					continue
				}
				if !first {
					sb.WriteString("\n")
				}
				first = false
				sb.WriteString(p.Key)
				sb.WriteString("=")
				if p.Type == "file" || (p.Type == "" && p.Src != "") {
					sb.WriteString("@")
					sb.WriteString(p.Src)
					if p.ContentType != "" {
						sb.WriteString(";type=")
						sb.WriteString(p.ContentType)
					}
				} else {
					sb.WriteString(p.Value)
				}
			}
			body = sb.String()
			bodyType = history.BodyMultipart
		case "graphql":
			if req.Body.GraphQL != nil {
				body = req.Body.GraphQL.Query
				graphqlVariables = normalisePostmanGraphQLVariables(req.Body.GraphQL.Variables)
				bodyType = history.BodyGraphQL
			}
		}
	}

	name := item.Name
	if name == "" {
		rawURL := req.URL.Raw
		if rawURL == "" {
			rawURL = "(no URL)"
		}
		name = method + " " + rawURL
	}
	return collections.Entry{
		ID:   uuid.NewString(),
		Name: name,
		Request: history.Request{
			Method:           method,
			URL:              req.URL.Raw,
			Headers:          headers,
			Body:             body,
			BodyType:         bodyType,
			GraphQLVariables: graphqlVariables,
		},
	}
}
