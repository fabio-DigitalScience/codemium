// internal/health/summary.go
package health

import (
	"github.com/dsablic/codemium/internal/model"
)

// Summarize aggregates health data across all repos into a HealthSummary.
// Returns nil if no repo has health data.
func Summarize(repos []model.RepoStats) *model.HealthSummary {
	var hasHealth bool
	summary := &model.HealthSummary{}
	var totalCode int64

	for _, r := range repos {
		if r.Health == nil {
			continue
		}
		hasHealth = true

		switch r.Health.Category {
		case model.HealthActive:
			summary.Active.Repos++
			summary.Active.Code += r.Totals.Code
			totalCode += r.Totals.Code
		case model.HealthMaintained:
			summary.Maintained.Repos++
			summary.Maintained.Code += r.Totals.Code
			totalCode += r.Totals.Code
		case model.HealthAbandoned:
			summary.Abandoned.Repos++
			summary.Abandoned.Code += r.Totals.Code
			totalCode += r.Totals.Code
		case model.HealthFailed:
			summary.Failed.Repos++
			summary.Failed.Code += r.Totals.Code
		}
	}

	if !hasHealth {
		return nil
	}

	if totalCode > 0 {
		summary.Active.CodePercent = float64(summary.Active.Code) / float64(totalCode) * 100
		summary.Maintained.CodePercent = float64(summary.Maintained.Code) / float64(totalCode) * 100
		summary.Abandoned.CodePercent = float64(summary.Abandoned.Code) / float64(totalCode) * 100
	}

	return summary
}
