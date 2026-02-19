// internal/output/json.go
package output

import (
	"encoding/json"
	"io"

	"github.com/dsablic/codemium/internal/model"
)

// WriteJSON writes the report as pretty-printed JSON to w.
func WriteJSON(w io.Writer, report model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteTrendsJSON writes the trends report as pretty-printed JSON to w.
func WriteTrendsJSON(w io.Writer, report model.TrendsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
