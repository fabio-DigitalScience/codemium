// internal/provider/gitlab_test.go
package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

func TestGitLabListRepos(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		if page == 1 {
			w.Header().Set("X-Next-Page", "2")
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":                  1,
					"path":                "repo-1",
					"path_with_namespace": "mygroup/repo-1",
					"name":                "Repo 1",
					"web_url":             "https://gitlab.com/mygroup/repo-1",
					"http_url_to_repo":    "https://gitlab.com/mygroup/repo-1.git",
					"default_branch":      "main",
					"archived":            false,
					"forked_from_project": nil,
					"namespace":           map[string]any{"full_path": "mygroup"},
				},
			})
		} else {
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":                  2,
					"path":                "repo-2",
					"path_with_namespace": "mygroup/repo-2",
					"name":                "Repo 2",
					"web_url":             "https://gitlab.com/mygroup/repo-2",
					"http_url_to_repo":    "https://gitlab.com/mygroup/repo-2.git",
					"default_branch":      "develop",
					"archived":            false,
					"forked_from_project": nil,
					"namespace":           map[string]any{"full_path": "mygroup"},
				},
			})
		}
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	repos, err := gl.ListRepos(context.Background(), provider.ListOpts{
		Organization: "mygroup",
	})
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Slug != "repo-1" {
		t.Errorf("expected repo-1, got %s", repos[0].Slug)
	}
	if repos[1].Slug != "repo-2" {
		t.Errorf("expected repo-2, got %s", repos[1].Slug)
	}
	if repos[0].Provider != "gitlab" {
		t.Errorf("expected provider gitlab, got %s", repos[0].Provider)
	}
	if repos[0].Project != "mygroup" {
		t.Errorf("expected project mygroup, got %s", repos[0].Project)
	}
}

func TestGitLabExcludeForksAndArchived(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": 1, "path": "active", "path_with_namespace": "g/active",
				"name": "Active", "web_url": "h", "http_url_to_repo": "c",
				"default_branch": "main", "archived": false, "forked_from_project": nil,
				"namespace": map[string]any{"full_path": "g"},
			},
			{
				"id": 2, "path": "archived-repo", "path_with_namespace": "g/archived-repo",
				"name": "Archived", "web_url": "h", "http_url_to_repo": "c",
				"default_branch": "main", "archived": true, "forked_from_project": nil,
				"namespace": map[string]any{"full_path": "g"},
			},
			{
				"id": 3, "path": "forked-repo", "path_with_namespace": "g/forked-repo",
				"name": "Forked", "web_url": "h", "http_url_to_repo": "c",
				"default_branch": "main", "archived": false,
				"forked_from_project": map[string]any{"id": 99},
				"namespace":           map[string]any{"full_path": "g"},
			},
		})
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	repos, _ := gl.ListRepos(context.Background(), provider.ListOpts{
		Organization:    "g",
		IncludeArchived: true, // server-side filter disabled so we test client-side fork filter
	})
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos (forks excluded), got %d", len(repos))
	}
}

func TestGitLabIncludeSlugFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "path": "repo-a", "path_with_namespace": "g/repo-a", "name": "A", "web_url": "h", "http_url_to_repo": "c", "default_branch": "main", "archived": false, "forked_from_project": nil, "namespace": map[string]any{"full_path": "g"}},
			{"id": 2, "path": "repo-b", "path_with_namespace": "g/repo-b", "name": "B", "web_url": "h", "http_url_to_repo": "c", "default_branch": "main", "archived": false, "forked_from_project": nil, "namespace": map[string]any{"full_path": "g"}},
		})
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	repos, _ := gl.ListRepos(context.Background(), provider.ListOpts{
		Organization: "g",
		Repos:        []string{"repo-a"},
	})
	if len(repos) != 1 || repos[0].Slug != "repo-a" {
		t.Fatalf("expected only repo-a, got %v", repos)
	}
}

func TestGitLabExcludeSlugFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "path": "repo-a", "path_with_namespace": "g/repo-a", "name": "A", "web_url": "h", "http_url_to_repo": "c", "default_branch": "main", "archived": false, "forked_from_project": nil, "namespace": map[string]any{"full_path": "g"}},
			{"id": 2, "path": "repo-b", "path_with_namespace": "g/repo-b", "name": "B", "web_url": "h", "http_url_to_repo": "c", "default_branch": "main", "archived": false, "forked_from_project": nil, "namespace": map[string]any{"full_path": "g"}},
		})
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	repos, _ := gl.ListRepos(context.Background(), provider.ListOpts{
		Organization: "g",
		Exclude:      []string{"repo-b"},
	})
	if len(repos) != 1 || repos[0].Slug != "repo-a" {
		t.Fatalf("expected only repo-a, got %v", repos)
	}
}

func TestGitLabAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "my-pat" {
			t.Errorf("expected PRIVATE-TOKEN header my-pat, got %q", got)
		}
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	gl := provider.NewGitLab("my-pat", server.URL)
	gl.ListRepos(context.Background(), provider.ListOpts{Organization: "g"})
}

func TestGitLabSelfHosted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v4/groups/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	_, err := gl.ListRepos(context.Background(), provider.ListOpts{Organization: "mygroup"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitLabListCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repository/commits") && !strings.Contains(r.URL.Path, "/repository/commits/") {
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":             "abc123",
					"author_name":    "Dev",
					"author_email":   "dev@example.com",
					"committed_date": "2025-06-15T10:30:00.000Z",
					"message":        "feat: add feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>",
				},
				{
					"id":             "def456",
					"author_name":    "Dev",
					"author_email":   "dev@example.com",
					"committed_date": "2025-06-14T09:00:00.000Z",
					"message":        "fix: bug",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	commits, err := gl.ListCommits(context.Background(), model.Repo{
		Slug: "repo-1",
		URL:  server.URL + "/mygroup/repo-1",
	}, 100)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "abc123" {
		t.Errorf("expected hash abc123, got %s", commits[0].Hash)
	}
	if !strings.Contains(commits[0].Message, "Co-Authored-By") {
		t.Error("expected full commit message with trailers")
	}
	if commits[0].Date.IsZero() {
		t.Error("expected commit date to be parsed")
	}
	if commits[0].Date.Year() != 2025 || commits[0].Date.Month() != 6 || commits[0].Date.Day() != 15 {
		t.Errorf("expected date 2025-06-15, got %s", commits[0].Date)
	}
}

func TestGitLabCommitStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/repository/commits/abc123") {
			json.NewEncoder(w).Encode(map[string]any{
				"id": "abc123",
				"stats": map[string]any{
					"additions": 150,
					"deletions": 30,
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	additions, deletions, err := gl.CommitStats(context.Background(), model.Repo{
		Slug: "repo-1",
		URL:  server.URL + "/mygroup/repo-1",
	}, "abc123")
	if err != nil {
		t.Fatalf("CommitStats: %v", err)
	}
	if additions != 150 {
		t.Errorf("expected 150 additions, got %d", additions)
	}
	if deletions != 30 {
		t.Errorf("expected 30 deletions, got %d", deletions)
	}
}

func TestGitLabListCommitsLimit(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		commits := make([]map[string]any, 100)
		for i := range commits {
			commits[i] = map[string]any{
				"id":           fmt.Sprintf("hash-%d-%d", page, i),
				"author_name":  "Dev",
				"author_email": "d@e.com",
				"message":      "msg",
			}
		}
		if page == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		json.NewEncoder(w).Encode(commits)
	}))
	defer server.Close()

	gl := provider.NewGitLab("test-token", server.URL)
	commits, err := gl.ListCommits(context.Background(), model.Repo{
		Slug: "repo-1",
		URL:  server.URL + "/mygroup/repo-1",
	}, 150)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 150 {
		t.Errorf("expected 150 commits (limited), got %d", len(commits))
	}
}
