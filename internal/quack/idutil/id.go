package idutil

import (
	"github.com/oklog/ulid/v2"
)

// NewULID creates a sortable unique identifier so domain and persistence records use one consistent ID format.
func NewULID() (string, error) {
	return ulid.Make().String(), nil
}
