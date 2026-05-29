package exporter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
	"gopkg.in/yaml.v3"
)

// OpenAPIFormat selects the serialisation format for ExportOpenAPI.
type OpenAPIFormat int

const (
	OpenAPIJSON OpenAPIFormat = iota
	OpenAPIYAML
)

const openAPIVersion = "3.0.3"

type openAPIDoc struct {
	OpenAPI string                 `json:"openapi" yaml:"openapi"`
	Info    openAPIInfo            `json:"info" yaml:"info"`
	Servers []openAPIServer        `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths   map[string]openAPIPath `json:"paths" yaml:"paths"`
}

type openAPIInfo struct {
	Title   string `json:"title" yaml:"title"`
	Version string `json:"version" yaml:"version"`
}

type openAPIServer struct {
	URL string `json:"url" yaml:"url"`
}

// openAPIPath declares operations using explicit struct fields so the
// rendered key order (get, post, put, ...) is deterministic across both
// encoding/json and gopkg.in/yaml.v3.
type openAPIPath struct {
	Get     *openAPIOp `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *openAPIOp `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *openAPIOp `json:"put,omitempty" yaml:"put,omitempty"`
	Patch   *openAPIOp `json:"patch,omitempty" yaml:"patch,omitempty"`
	Delete  *openAPIOp `json:"delete,omitempty" yaml:"delete,omitempty"`
	Head    *openAPIOp `json:"head,omitempty" yaml:"head,omitempty"`
	Options *openAPIOp `json:"options,omitempty" yaml:"options,omitempty"`
	Trace   *openAPIOp `json:"trace,omitempty" yaml:"trace,omitempty"`
}

type openAPIOp struct {
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Parameters  []openAPIParameter  `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *openAPIRequestBody `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
}

type openAPIParameter struct {
	Name    string        `json:"name" yaml:"name"`
	In      string        `json:"in" yaml:"in"`
	Schema  openAPISchema `json:"schema" yaml:"schema"`
	Example string        `json:"example,omitempty" yaml:"example,omitempty"`
}

type openAPISchema struct {
	Type       string                   `json:"type,omitempty" yaml:"type,omitempty"`
	Properties map[string]openAPISchema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Example    string                   `json:"example,omitempty" yaml:"example,omitempty"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `json:"content" yaml:"content"`
}

type openAPIMediaType struct {
	Schema  *openAPISchema `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example string         `json:"example,omitempty" yaml:"example,omitempty"`
}

// ExportOpenAPI serialises entries as an OpenAPI 3.0.3 document. When format
// is OpenAPIYAML the JSON output is re-serialised as YAML so map key ordering
// stays identical (encoding/json sorts map keys alphabetically; yaml.Node
// preserves source order).
func ExportOpenAPI(entries []collections.Entry, title string, format OpenAPIFormat) ([]byte, error) {
	doc := buildOpenAPIDoc(entries, title)

	jsonBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	if format == OpenAPIJSON {
		return jsonBytes, nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal(jsonBytes, &node); err != nil {
		return nil, fmt.Errorf("yaml convert: %w", err)
	}
	// yaml.Unmarshal of JSON marks every node FlowStyle, which round-trips
	// back out as `{...}` / `[...]`. Strip the style so Marshal renders
	// human-readable block YAML while keeping the JSON insertion order.
	clearYAMLFlowStyle(&node)
	out, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal: %w", err)
	}
	return out, nil
}

func clearYAMLFlowStyle(n *yaml.Node) {
	if n == nil {
		return
	}
	n.Style = 0
	for _, c := range n.Content {
		clearYAMLFlowStyle(c)
	}
}

func buildOpenAPIDoc(entries []collections.Entry, title string) openAPIDoc {
	if title == "" {
		title = "pollen"
	}
	doc := openAPIDoc{
		OpenAPI: openAPIVersion,
		Info:    openAPIInfo{Title: title, Version: "1.0.0"},
		Paths:   map[string]openAPIPath{},
	}

	server, useServer := chooseServer(entries)
	if useServer {
		doc.Servers = []openAPIServer{{URL: server}}
	}

	usedIDs := map[string]int{}

	// Sort entries by name to make operationId deduplication and last-wins
	// behaviour deterministic.
	sorted := make([]collections.Entry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, e := range sorted {
		path, query := splitPath(e.Request.URL, server, useServer)
		method := strings.ToLower(strings.TrimSpace(e.Request.Method))
		if method == "" {
			method = "get"
		}
		op := entryToOperation(e, query, usedIDs)
		assignOperation(doc.Paths, path, method, op)
	}

	return doc
}

// chooseServer returns the scheme://host shared by every parseable entry URL
// and true when a single host covers them all. Mixed hosts, template tokens,
// or unparseable URLs fall back to returning false.
func chooseServer(entries []collections.Entry) (string, bool) {
	hosts := map[string]struct{}{}
	for _, e := range entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", false
		}
		hosts[u.Scheme+"://"+u.Host] = struct{}{}
	}
	if len(hosts) != 1 {
		return "", false
	}
	for h := range hosts {
		return h, true
	}
	return "", false
}

// splitPath returns the path component and any query pairs for an entry URL.
// When a server is in play we strip the scheme://host prefix; otherwise the
// raw URL becomes the path (forward-slash-prefixed so the OpenAPI paths key
// remains syntactically valid for static analysers).
func splitPath(raw, server string, useServer bool) (path string, query []openAPIParameter) {
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Host != "" {
		path = u.Path
		if path == "" {
			path = "/"
		}
		query = queryParams(u.RawQuery)
		if useServer {
			return path, query
		}
		// Mixed-host export: keep the full URL so the spec still
		// records which host this operation hits.
		return ensureLeadingSlash(raw), query
	}
	// Unparseable (variable tokens, opaque strings) — emit as-is.
	if idx := strings.Index(raw, "?"); idx >= 0 {
		query = queryParams(raw[idx+1:])
		raw = raw[:idx]
	}
	return ensureLeadingSlash(raw), query
}

func ensureLeadingSlash(s string) string {
	if strings.HasPrefix(s, "/") {
		return s
	}
	return "/" + s
}

func queryParams(rawQuery string) []openAPIParameter {
	if rawQuery == "" {
		return nil
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]openAPIParameter, 0, len(names))
	for _, name := range names {
		val := ""
		if vs := values[name]; len(vs) > 0 {
			val = vs[0]
		}
		out = append(out, openAPIParameter{
			Name:    name,
			In:      "query",
			Schema:  openAPISchema{Type: "string"},
			Example: val,
		})
	}
	return out
}

func entryToOperation(e collections.Entry, query []openAPIParameter, usedIDs map[string]int) *openAPIOp {
	op := &openAPIOp{
		Summary:     e.Name,
		OperationID: uniqueOperationID(e.Name, usedIDs),
	}
	op.Parameters = append(op.Parameters, query...)
	for _, h := range e.Request.Headers {
		if strings.TrimSpace(h.Key) == "" {
			continue
		}
		op.Parameters = append(op.Parameters, openAPIParameter{
			Name:    h.Key,
			In:      "header",
			Schema:  openAPISchema{Type: "string"},
			Example: h.Value,
		})
	}
	if e.Request.Body != "" {
		op.RequestBody = buildRequestBody(e.Request)
	}
	return op
}

func buildRequestBody(req history.Request) *openAPIRequestBody {
	switch req.BodyType {
	case history.BodyForm:
		props := map[string]openAPISchema{}
		for _, p := range parseFormPairs(req.Body) {
			props[p.Key] = openAPISchema{Type: "string", Example: p.Value}
		}
		return &openAPIRequestBody{
			Content: map[string]openAPIMediaType{
				"application/x-www-form-urlencoded": {
					Schema: &openAPISchema{
						Type:       "object",
						Properties: props,
					},
				},
			},
		}
	case history.BodyJSON:
		return &openAPIRequestBody{
			Content: map[string]openAPIMediaType{
				"application/json": {Example: req.Body},
			},
		}
	default:
		// BodyRaw: prefer an explicit Content-Type header if present so the
		// spec records the same media type the runtime would actually send.
		ct := "text/plain"
		for _, h := range req.Headers {
			if strings.EqualFold(h.Key, "Content-Type") && strings.TrimSpace(h.Value) != "" {
				ct = strings.TrimSpace(h.Value)
				break
			}
		}
		return &openAPIRequestBody{
			Content: map[string]openAPIMediaType{
				ct: {Example: req.Body},
			},
		}
	}
}

// uniqueOperationID derives an operationId from a free-form name and ensures
// it does not collide with one already emitted. Collisions get a numeric
// suffix (`getUsers`, `getUsers_2`, `getUsers_3`, ...).
func uniqueOperationID(name string, used map[string]int) string {
	base := sanitizeOperationID(name)
	if _, taken := used[base]; !taken {
		used[base] = 1
		return base
	}
	used[base]++
	return fmt.Sprintf("%s_%d", base, used[base])
}

func sanitizeOperationID(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case r >= '0' && r <= '9' && i > 0:
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "op"
	}
	return b.String()
}

// assignOperation slots op into the right method field on the path entry,
// creating the path entry on first sight. Unknown methods are ignored so the
// caller is responsible for normalising req.Method beforehand.
func assignOperation(paths map[string]openAPIPath, path, method string, op *openAPIOp) {
	p := paths[path]
	switch method {
	case "get":
		p.Get = op
	case "post":
		p.Post = op
	case "put":
		p.Put = op
	case "patch":
		p.Patch = op
	case "delete":
		p.Delete = op
	case "head":
		p.Head = op
	case "options":
		p.Options = op
	case "trace":
		p.Trace = op
	default:
		// Unknown verb — fall back to GET so the entry isn't silently dropped.
		p.Get = op
	}
	paths[path] = p
}
