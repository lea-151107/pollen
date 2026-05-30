// Package exporter converts pollen collections to external formats.
package exporter

import (
	"encoding/json"
	"strings"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/history"
)

type postmanExportCollection struct {
	Info postmanExportInfo   `json:"info"`
	Item []postmanExportItem `json:"item"`
}

type postmanExportInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type postmanExportItem struct {
	Name    string           `json:"name"`
	Request postmanExportReq `json:"request"`
}

type postmanExportReq struct {
	Method string              `json:"method"`
	URL    map[string]string   `json:"url"`
	Header []map[string]string `json:"header,omitempty"`
	Body   *postmanExportBody  `json:"body,omitempty"`
}

type postmanExportBody struct {
	Mode       string                   `json:"mode"`
	Raw        string                   `json:"raw,omitempty"`
	URLEncoded []postmanExportFormParam `json:"urlencoded,omitempty"`
	GraphQL    *postmanGraphQLBody      `json:"graphql,omitempty"`
}

type postmanGraphQLBody struct {
	Query     string `json:"query"`
	Variables string `json:"variables,omitempty"`
}

type postmanExportFormParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ExportPostman serialises entries as a Postman Collection v2.1 JSON document.
func ExportPostman(entries []collections.Entry, name string) ([]byte, error) {
	// Postman v2.1 requires `item` to be an array. Initialise the slice so
	// an empty collection serialises as `"item": []` instead of `"item": null`,
	// which strict parsers reject.
	col := postmanExportCollection{
		Info: postmanExportInfo{
			Name:   name,
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		Item: []postmanExportItem{},
	}
	for _, e := range entries {
		col.Item = append(col.Item, entryToItem(e))
	}
	return json.MarshalIndent(col, "", "  ")
}

func entryToItem(e collections.Entry) postmanExportItem {
	req := e.Request
	item := postmanExportItem{
		Name: e.Name,
		Request: postmanExportReq{
			Method: req.Method,
			URL:    map[string]string{"raw": req.URL},
		},
	}
	for _, h := range req.Headers {
		if h.Key == "" {
			continue
		}
		item.Request.Header = append(item.Request.Header, map[string]string{
			"key":   h.Key,
			"value": h.Value,
		})
	}
	if req.Body != "" || req.BodyType == history.BodyGraphQL {
		body := &postmanExportBody{}
		switch req.BodyType {
		case history.BodyForm:
			body.Mode = "urlencoded"
			body.URLEncoded = parseFormPairs(req.Body)
		case history.BodyGraphQL:
			// Postman v2.1 stores GraphQL variables as a string in the
			// "variables" field; pollen already keeps it that way.
			body.Mode = "graphql"
			body.GraphQL = &postmanGraphQLBody{
				Query:     req.Body,
				Variables: req.GraphQLVariables,
			}
		default:
			body.Mode = "raw"
			body.Raw = req.Body
		}
		item.Request.Body = body
	}
	return item
}

// parseFormPairs splits Pollen's internal "key=value\nkey=value" form body
// into Postman's urlencoded array structure. Mirrors the parser in
// httpx.buildBody so an export round-trips the same set of pairs the runtime
// would actually send.
func parseFormPairs(body string) []postmanExportFormParam {
	var out []postmanExportFormParam
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, postmanExportFormParam{
			Key:   strings.TrimSpace(k),
			Value: strings.TrimSpace(v),
		})
	}
	return out
}
