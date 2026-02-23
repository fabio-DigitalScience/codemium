// internal/health/health.go
package health

import (
	"time"

	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

const (
	ActiveThresholdDays     = 180
	MaintainedThresholdDays = 365
)

// Classify returns a RepoHealth based on the last commit date relative to now.
func Classify(lastCommitDate, now time.Time) *model.RepoHealth {
	days := int(now.Sub(lastCommitDate).Hours() / 24)
	if days < 0 {
		days = 0
	}

	var category model.HealthCategory
	switch {
	case days < ActiveThresholdDays:
		category = model.HealthActive
	case days < MaintainedThresholdDays:
		category = model.HealthMaintained
	default:
		category = model.HealthAbandoned
	}

	return &model.RepoHealth{
		Category:        category,
		LastCommitDate:  lastCommitDate.UTC().Format(time.RFC3339),
		DaysSinceCommit: days,
	}
}

// ClassifyFromCommits classifies a repo based on a list of commits.
// It finds the most recent commit date. Returns nil if commits is empty.
func ClassifyFromCommits(commits []provider.CommitInfo, now time.Time) *model.RepoHealth {
	if len(commits) == 0 {
		return nil
	}

	latest := commits[0].Date
	for _, c := range commits[1:] {
		if c.Date.After(latest) {
			latest = c.Date
		}
	}

	if latest.IsZero() {
		return nil
	}

	return Classify(latest, now)
}
