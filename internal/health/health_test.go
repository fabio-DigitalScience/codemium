package health

import (
	"context"
	"testing"
	"time"

	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

func TestClassifyBoundaries(t *testing.T) {
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		daysAgo  int
		expected model.HealthCategory
	}{
		{"179 days = active", 179, model.HealthActive},
		{"180 days = maintained", 180, model.HealthMaintained},
		{"364 days = maintained", 364, model.HealthMaintained},
		{"365 days = abandoned", 365, model.HealthAbandoned},
		{"366 days = abandoned", 366, model.HealthAbandoned},
		{"0 days = active", 0, model.HealthActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastCommit := now.AddDate(0, 0, -tt.daysAgo)
			result := Classify(lastCommit, now)
			if result.Category != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result.Category)
			}
			if result.DaysSinceCommit != tt.daysAgo {
				t.Errorf("expected %d days, got %d", tt.daysAgo, result.DaysSinceCommit)
			}
		})
	}
}

func TestClassifyFromCommitsEmpty(t *testing.T) {
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	result := ClassifyFromCommits(nil, now)
	if result == nil {
		t.Fatal("expected non-nil result for empty commits")
	}
	if result.Category != model.HealthAbandoned {
		t.Errorf("expected abandoned for empty commits, got %s", result.Category)
	}
	if result.DaysSinceCommit != -1 {
		t.Errorf("expected -1 days for empty commits, got %d", result.DaysSinceCommit)
	}
}

func TestClassifyFromCommits(t *testing.T) {
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	commits := []provider.CommitInfo{
		{Hash: "old", Date: now.AddDate(0, 0, -400)},
		{Hash: "recent", Date: now.AddDate(0, 0, -10)},
		{Hash: "middle", Date: now.AddDate(0, 0, -200)},
	}
	result := ClassifyFromCommits(commits, now)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Category != model.HealthActive {
		t.Errorf("expected active (most recent is 10 days), got %s", result.Category)
	}
}

func TestSummarizeMixed(t *testing.T) {
	repos := []model.RepoStats{
		{
			Repository: "active-repo",
			Totals:     model.Stats{Code: 1000},
			Health:     &model.RepoHealth{Category: model.HealthActive},
		},
		{
			Repository: "maintained-repo",
			Totals:     model.Stats{Code: 2000},
			Health:     &model.RepoHealth{Category: model.HealthMaintained},
		},
		{
			Repository: "abandoned-repo",
			Totals:     model.Stats{Code: 7000},
			Health:     &model.RepoHealth{Category: model.HealthAbandoned},
		},
	}

	summary := Summarize(repos)
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.Active.Repos != 1 {
		t.Errorf("expected 1 active repo, got %d", summary.Active.Repos)
	}
	if summary.Maintained.Repos != 1 {
		t.Errorf("expected 1 maintained repo, got %d", summary.Maintained.Repos)
	}
	if summary.Abandoned.Repos != 1 {
		t.Errorf("expected 1 abandoned repo, got %d", summary.Abandoned.Repos)
	}
	if summary.Active.Code != 1000 {
		t.Errorf("expected 1000 active code, got %d", summary.Active.Code)
	}
	// 1000 / 10000 = 10%
	if summary.Active.CodePercent != 10.0 {
		t.Errorf("expected 10%% active code, got %.1f%%", summary.Active.CodePercent)
	}
	if summary.Abandoned.CodePercent != 70.0 {
		t.Errorf("expected 70%% abandoned code, got %.1f%%", summary.Abandoned.CodePercent)
	}
}

func TestSummarizeWithFailed(t *testing.T) {
	repos := []model.RepoStats{
		{
			Repository: "active-repo",
			Totals:     model.Stats{Code: 1000},
			Health:     &model.RepoHealth{Category: model.HealthActive},
		},
		{
			Repository: "failed-repo",
			Totals:     model.Stats{Code: 500},
			Health:     &model.RepoHealth{Category: model.HealthFailed, DaysSinceCommit: -1},
		},
	}

	summary := Summarize(repos)
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.Active.Repos != 1 {
		t.Errorf("expected 1 active repo, got %d", summary.Active.Repos)
	}
	if summary.Failed.Repos != 1 {
		t.Errorf("expected 1 failed repo, got %d", summary.Failed.Repos)
	}
	if summary.Failed.Code != 500 {
		t.Errorf("expected 500 failed code, got %d", summary.Failed.Code)
	}
	if summary.Active.CodePercent != 100.0 {
		t.Errorf("expected 100%% active code (failed excluded from total), got %.1f%%", summary.Active.CodePercent)
	}
}

func TestSummarizeNoHealth(t *testing.T) {
	repos := []model.RepoStats{
		{Repository: "repo-1", Totals: model.Stats{Code: 1000}},
	}
	summary := Summarize(repos)
	if summary != nil {
		t.Error("expected nil summary when no health data")
	}
}

// mockCommitLister implements provider.CommitLister for testing.
type mockCommitLister struct {
	commits  []provider.CommitInfo
	statsMap map[string][2]int64 // hash -> [additions, deletions]
	statsErr error
}

func (m *mockCommitLister) ListCommits(_ context.Context, _ model.Repo, _ int) ([]provider.CommitInfo, error) {
	return m.commits, nil
}

func (m *mockCommitLister) CommitStats(_ context.Context, _ model.Repo, hash string) (int64, int64, error) {
	if m.statsErr != nil {
		return 0, 0, m.statsErr
	}
	if s, ok := m.statsMap[hash]; ok {
		return s[0], s[1], nil
	}
	return 0, 0, nil
}

func TestAnalyzeDetails(t *testing.T) {
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	commits := []provider.CommitInfo{
		{Hash: "a1", Author: "Alice <alice@example.com>", Date: now.AddDate(0, -1, 0)},
		{Hash: "a2", Author: "Bob <bob@example.com>", Date: now.AddDate(0, -2, 0)},
		{Hash: "a3", Author: "Alice <alice@example.com>", Date: now.AddDate(0, -8, 0)},
		{Hash: "a4", Author: "Charlie <charlie@example.com>", Date: now.AddDate(0, -15, 0)},
	}

	lister := &mockCommitLister{
		commits: commits,
		statsMap: map[string][2]int64{
			"a1": {100, 20},
			"a2": {50, 10},
			"a3": {200, 50},
			"a4": {30, 5},
		},
	}

	repo := model.Repo{Slug: "test-repo", URL: "https://github.com/org/test-repo"}
	details, err := AnalyzeDetails(context.Background(), lister, repo, commits, now)
	if err != nil {
		t.Fatalf("AnalyzeDetails: %v", err)
	}

	// 0-6mo: Alice, Bob (2 authors, 2 commits)
	if details.AuthorsByWindow[Window0to6] != 2 {
		t.Errorf("expected 2 authors in 0-6mo, got %d", details.AuthorsByWindow[Window0to6])
	}
	// 6-12mo: Alice (1 author, 1 commit)
	if details.AuthorsByWindow[Window6to12] != 1 {
		t.Errorf("expected 1 author in 6-12mo, got %d", details.AuthorsByWindow[Window6to12])
	}
	// 12mo+: Charlie (1 author, 1 commit)
	if details.AuthorsByWindow[Window12Plus] != 1 {
		t.Errorf("expected 1 author in 12mo+, got %d", details.AuthorsByWindow[Window12Plus])
	}

	// Bus factor: Alice has 2 commits out of 4 = 50%
	if details.BusFactor != 50.0 {
		t.Errorf("expected bus factor 50%%, got %.1f%%", details.BusFactor)
	}

	// Velocity: 2 commits (0-6mo) / 1 commit (6-12mo) = 2.0
	if details.VelocityTrend != 2.0 {
		t.Errorf("expected velocity trend 2.0, got %.1f", details.VelocityTrend)
	}

	// Churn: 0-6mo should have additions=150, deletions=30
	if cs, ok := details.ChurnByWindow[Window0to6]; ok {
		if cs.Additions != 150 {
			t.Errorf("expected 150 additions in 0-6mo, got %d", cs.Additions)
		}
		if cs.Deletions != 30 {
			t.Errorf("expected 30 deletions in 0-6mo, got %d", cs.Deletions)
		}
		if cs.NetChurn != 120 {
			t.Errorf("expected 120 net churn in 0-6mo, got %d", cs.NetChurn)
		}
	} else {
		t.Error("expected churn data for 0-6mo window")
	}
}

func TestAnalyzeDetailsEmpty(t *testing.T) {
	now := time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC)
	lister := &mockCommitLister{}
	repo := model.Repo{Slug: "empty-repo"}

	details, err := AnalyzeDetails(context.Background(), lister, repo, nil, now)
	if err != nil {
		t.Fatalf("AnalyzeDetails: %v", err)
	}
	if details == nil {
		t.Fatal("expected non-nil details for empty commits")
	}
	if len(details.AuthorsByWindow) != 0 {
		t.Errorf("expected empty authors, got %v", details.AuthorsByWindow)
	}
}
