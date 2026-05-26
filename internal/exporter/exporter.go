// Package exporter converts pollen collections to external formats.
package exporter

import (
	"encoding/json"

	"github.com/lea/pollen/internal/collections"
	"github.com/lea/pollen/internal/history"
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
	Mode string `json:"mode"`
	Raw  string `json:"raw,omitempty"`
}

// ExportPostman serialises entries as a Postman Collection v2.1 JSON document.
func ExportPostman(entries []collections.Entry, name string) ([]byte, error) {
	col := postmanExportCollection{
		Info: postmanExportInfo{
			Name:   name,
			Schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
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
	if req.Body != "" {
		body := &postmanExportBody{}
		switch req.BodyType {
		case history.BodyForm:
			body.Mode = "urlencoded"
			body.Raw = req.Body
		default:
			body.Mode = "raw"
			body.Raw = req.Body
		}
		item.Request.Body = body
	}
	return item
}
