package license

import (
	"github.com/go-enry/go-license-detector/v4/licensedb"
	"github.com/go-enry/go-license-detector/v4/licensedb/filer"
)

const confidenceThreshold = 0.85

// Detect scans the directory for license files and returns the SPDX
// identifier of the most confident match, or empty string if none found.
func Detect(dir string) string {
	f, err := filer.FromDirectory(dir)
	if err != nil {
		return ""
	}

	results, err := licensedb.Detect(f)
	if err != nil {
		return ""
	}

	var bestID string
	var bestConf float32
	for id, match := range results {
		if match.Confidence > bestConf && match.Confidence >= confidenceThreshold {
			bestConf = match.Confidence
			bestID = id
		}
	}

	return bestID
}
