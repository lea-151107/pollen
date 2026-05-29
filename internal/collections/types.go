// Package collections persists named saved requests in
// ~/.config/pollen/collections.json and exposes Add / Delete / Rename /
// UpdateRequest operations on the store.
package collections

import "github.com/lea-151107/pollen/internal/history"

type Entry struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Request history.Request `json:"request"`
}

type File struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}
