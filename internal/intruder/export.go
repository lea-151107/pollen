package intruder

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CSV serialises results as RFC 4180 CSV with a header row. Useful for
// piping into spreadsheets or grep-ing for outliers.
func CSV(results []Result) ([]byte, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	header := []string{"index", "payload", "status", "status_text", "size_bytes", "duration_ms", "content_type", "error"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, r := range results {
		row := []string{
			strconv.Itoa(r.Index),
			r.Payload,
			strconv.Itoa(r.Status),
			r.StatusText,
			strconv.Itoa(r.Size),
			strconv.FormatInt(r.DurationMs, 10),
			r.ContentType,
			r.Error,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	return []byte(sb.String()), nil
}

// JSON serialises results as an indented JSON array, mirroring the
// Postman / OpenAPI exporters' shape so a downstream tool can rely on
// "indented JSON" from pollen's export commands across the board.
func JSON(results []Result) ([]byte, error) {
	// Initialise a zero-length slice so the empty case marshals as `[]`
	// rather than `null`, matching the Postman exporter's invariant.
	rows := results
	if rows == nil {
		rows = []Result{}
	}
	return json.MarshalIndent(rows, "", "  ")
}
