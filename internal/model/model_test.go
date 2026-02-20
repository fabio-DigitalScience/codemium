// internal/model/model_test.go
package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dsablic/codemium/internal/model"
)

func TestReportJSON(t *testing.T) {
	report := model.Report{
		GeneratedAt: "2026-02-18T12:00:00Z",
		Provider:    "bitbucket",
		Workspace:   "myworkspace",
		Filters:     model.Filters{Projects: []string{"PROJ1"}},
		Repositories: []model.RepoStats{
			{
				Repository: "my-repo",
				Project:    "PROJ1",
				Provider:   "bitbucket",
				URL:        "https://bitbucket.org/myworkspace/my-repo",
				Languages: []model.LanguageStats{
					{Name: "Go", Files: 10, Lines: 500, Code: 400, Comments: 50, Blanks: 50, Complexity: 30},
				},
				Totals: model.Stats{Files: 10, Lines: 500, Code: 400, Comments: 50, Blanks: 50, Complexity: 30},
			},
		},
		Totals: model.Stats{Repos: 1, Files: 10, Lines: 500, Code: 400, Comments: 50, Blanks: 50, Complexity: 30},
		ByLanguage: []model.LanguageStats{
			{Name: "Go", Files: 10, Lines: 500, Code: 400, Comments: 50, Blanks: 50, Complexity: 30},
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}

	var decoded model.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if decoded.Provider != "bitbucket" {
		t.Errorf("expected provider bitbucket, got %s", decoded.Provider)
	}
	if decoded.Totals.Code != 400 {
		t.Errorf("expected total code 400, got %d", decoded.Totals.Code)
	}
	if len(decoded.Repositories) != 1 {
		t.Errorf("expected 1 repo, got %d", len(decoded.Repositories))
	}
	if decoded.Repositories[0].Project != "PROJ1" {
		t.Errorf("expected project PROJ1, got %s", decoded.Repositories[0].Project)
	}
}

func TestAIEstimateJSONOmitsWhenNil(t *testing.T) {
	report := model.Report{
		GeneratedAt: "2026-02-20T12:00:00Z",
		Provider:    "github",
		Totals:      model.Stats{Repos: 1, Code: 100},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(data), "ai_estimate") {
		t.Error("expected ai_estimate to be omitted when nil")
	}
}

func TestAIEstimateJSONRoundTrip(t *testing.T) {
	estimate := &model.AIEstimate{
		TotalCommits:    100,
		AICommits:       25,
		CommitPercent:   25.0,
		TotalAdditions:  5000,
		AIAdditions:     1500,
		AdditionPercent: 30.0,
		Details: []model.AICommit{
			{
				Hash:      "abc123",
				Author:    "Claude <noreply@anthropic.com>",
				Message:   "feat: add feature",
				Signals:   []model.AISignal{model.SignalCoAuthor},
				Additions: 100,
				Deletions: 10,
			},
		},
	}

	report := model.Report{
		GeneratedAt: "2026-02-20T12:00:00Z",
		Provider:    "github",
		Totals:      model.Stats{Repos: 1, Code: 100},
		AIEstimate:  estimate,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded model.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.AIEstimate == nil {
		t.Fatal("expected AIEstimate to be present")
	}
	if decoded.AIEstimate.AICommits != 25 {
		t.Errorf("expected 25 AI commits, got %d", decoded.AIEstimate.AICommits)
	}
	if decoded.AIEstimate.CommitPercent != 25.0 {
		t.Errorf("expected 25.0%% commit percent, got %f", decoded.AIEstimate.CommitPercent)
	}
	if len(decoded.AIEstimate.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(decoded.AIEstimate.Details))
	}
	if decoded.AIEstimate.Details[0].Signals[0] != model.SignalCoAuthor {
		t.Errorf("expected co-author signal, got %s", decoded.AIEstimate.Details[0].Signals[0])
	}
}

func TestRepoStatsAIEstimateOmitsWhenNil(t *testing.T) {
	stats := model.RepoStats{
		Repository: "test",
		Provider:   "github",
		Totals:     model.Stats{Code: 100},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(data), "ai_estimate") {
		t.Error("expected ai_estimate to be omitted when nil")
	}
}

func TestTrendsReportJSON(t *testing.T) {
	report := model.TrendsReport{
		GeneratedAt:  "2026-02-19T12:00:00Z",
		Provider:     "github",
		Organization: "myorg",
		Since:        "2025-01",
		Until:        "2025-03",
		Interval:     "monthly",
		Periods:      []string{"2025-01", "2025-02", "2025-03"},
		Snapshots: []model.PeriodSnapshot{
			{
				Period: "2025-01",
				Repositories: []model.RepoStats{
					{
						Repository: "my-repo",
						Provider:   "github",
						URL:        "https://github.com/myorg/my-repo",
						Totals:     model.Stats{Files: 10, Code: 400},
					},
				},
				Totals:     model.Stats{Repos: 1, Files: 10, Code: 400},
				ByLanguage: []model.LanguageStats{{Name: "Go", Files: 10, Code: 400}},
			},
		},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal trends report: %v", err)
	}

	var decoded model.TrendsReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal trends report: %v", err)
	}

	if decoded.Interval != "monthly" {
		t.Errorf("expected interval monthly, got %s", decoded.Interval)
	}
	if len(decoded.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(decoded.Snapshots))
	}
	if decoded.Snapshots[0].Totals.Code != 400 {
		t.Errorf("expected code 400, got %d", decoded.Snapshots[0].Totals.Code)
	}
}
