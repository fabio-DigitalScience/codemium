package churn_test

import (
	"context"
	"testing"

	"github.com/dsablic/codemium/internal/churn"
	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

type mockChurnLister struct {
	commits []provider.CommitInfo
	files   map[string][]provider.FileChange
}

func (m *mockChurnLister) ListCommits(_ context.Context, _ model.Repo, limit int) ([]provider.CommitInfo, error) {
	if limit > 0 && limit < len(m.commits) {
		return m.commits[:limit], nil
	}
	return m.commits, nil
}

func (m *mockChurnLister) CommitStats(_ context.Context, _ model.Repo, _ string) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockChurnLister) CommitFileStats(_ context.Context, _ model.Repo, hash string) ([]provider.FileChange, error) {
	return m.files[hash], nil
}

func TestAnalyzeChurn(t *testing.T) {
	mock := &mockChurnLister{
		commits: []provider.CommitInfo{{Hash: "aaa"}, {Hash: "bbb"}, {Hash: "ccc"}},
		files: map[string][]provider.FileChange{
			"aaa": {{Path: "main.go", Additions: 50, Deletions: 10}, {Path: "util.go", Additions: 20, Deletions: 5}},
			"bbb": {{Path: "main.go", Additions: 30, Deletions: 5}},
			"ccc": {{Path: "main.go", Additions: 10, Deletions: 2}, {Path: "README.md", Additions: 5, Deletions: 0}},
		},
	}

	stats, err := churn.Analyze(context.Background(), mock, model.Repo{Slug: "test"}, 0)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if stats.TotalCommits != 3 {
		t.Errorf("expected 3 total commits, got %d", stats.TotalCommits)
	}
	if len(stats.TopFiles) == 0 {
		t.Fatal("expected at least 1 top file")
	}
	if stats.TopFiles[0].Path != "main.go" {
		t.Errorf("expected main.go as top file, got %s", stats.TopFiles[0].Path)
	}
	if stats.TopFiles[0].Changes != 3 {
		t.Errorf("expected 3 changes for main.go, got %d", stats.TopFiles[0].Changes)
	}
}

func TestAnalyzeChurnLimit(t *testing.T) {
	mock := &mockChurnLister{
		commits: []provider.CommitInfo{{Hash: "aaa"}, {Hash: "bbb"}},
		files: map[string][]provider.FileChange{
			"aaa": {{Path: "main.go", Additions: 50, Deletions: 10}},
			"bbb": {{Path: "main.go", Additions: 30, Deletions: 5}},
		},
	}

	stats, err := churn.Analyze(context.Background(), mock, model.Repo{Slug: "test"}, 1)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if stats.TotalCommits != 1 {
		t.Errorf("expected 1 commit (limited), got %d", stats.TotalCommits)
	}
}

func TestComputeHotspots(t *testing.T) {
	files := []model.FileChurn{
		{Path: "complex.go", Changes: 10, Additions: 500, Deletions: 100},
		{Path: "simple.go", Changes: 20, Additions: 200, Deletions: 50},
		{Path: "util.go", Changes: 5, Additions: 100, Deletions: 20},
	}
	complexity := map[string]int64{"complex.go": 50, "simple.go": 2, "util.go": 10}

	hotspots := churn.ComputeHotspots(files, complexity, 10)
	if len(hotspots) != 3 {
		t.Fatalf("expected 3 hotspots, got %d", len(hotspots))
	}
	if hotspots[0].Path != "complex.go" {
		t.Errorf("expected complex.go as top hotspot, got %s", hotspots[0].Path)
	}
	if hotspots[0].Complexity != 50 {
		t.Errorf("expected complexity 50, got %d", hotspots[0].Complexity)
	}
	if hotspots[0].Hotspot != 500 {
		t.Errorf("expected hotspot score 500, got %f", hotspots[0].Hotspot)
	}
}
