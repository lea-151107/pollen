package app

import (
	"testing"

	"github.com/lea-151107/pollen/internal/history"
)

func TestExpandResponseVars(t *testing.T) {
	resp := &history.Response{
		Status:     200,
		Body:       `{"token":"abc123","user":{"id":42}}`,
		Headers:    []history.Header{{Key: "Content-Type", Value: "application/json"}},
		ContentType: "application/json",
	}

	tests := []struct {
		name  string
		input string
		resp  *history.Response
		want  string
	}{
		{"nil resp leaves token", "Bearer {{response.body.token}}", nil, "Bearer {{response.body.token}}"},
		{"status", "{{response.status}}", resp, "200"},
		{"body field", "{{response.body.token}}", resp, "abc123"},
		{"nested body field", "{{response.body.user.id}}", resp, "42"},
		{"whole body", "{{response.body}}", resp, resp.Body},
		{"header case-insensitive", "{{response.headers.content-type}}", resp, "application/json"},
		{"header original case", "{{response.headers.Content-Type}}", resp, "application/json"},
		{"unknown header leaves token", "{{response.headers.x-missing}}", resp, "{{response.headers.x-missing}}"},
		{"missing field returns empty string", "{{response.body.missing}}", resp, ""},
		{"multiple tokens", "{{response.status}} {{response.body.token}}", resp, "200 abc123"},
		{"no response tokens unchanged", "hello world", resp, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandResponseVars(tt.input, tt.resp)
			if got != tt.want {
				t.Errorf("expandResponseVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
