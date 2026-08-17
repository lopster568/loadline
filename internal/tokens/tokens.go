// Package tokens holds one adapter per provider tokenizer. An adapter that
// has no credential reports itself unavailable rather than estimating; a cell
// without a real count is never published (methodology 1.7).
package tokens

import (
	"errors"
	"time"
)

// ErrUnavailable is returned by an adapter with no usable credential.
var ErrUnavailable = errors.New("tokenizer unavailable: no credential configured")

// Stamp is the per-cell provenance required by methodology 1.7.
type Stamp struct {
	Model      string    `json:"model,omitempty"`
	Encoding   string    `json:"encoding,omitempty"`
	MeasuredAt time.Time `json:"measured_at"`
}

func nowUTC() time.Time { return time.Now().UTC() }
