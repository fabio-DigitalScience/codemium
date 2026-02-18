# Project Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an interactive Bitbucket project picker that appears when `--projects` is not specified in a TTY.

**Architecture:** Add `ListProjects` to the Bitbucket provider (paginated API call to `/2.0/workspaces/{workspace}/projects`). Add a `PickProjects` function in `internal/ui/` using `charmbracelet/huh` multi-select with a "Select All" toggle. Wire into `runAnalyze` between provider creation and `ListRepos`.

**Tech Stack:** Go, charmbracelet/huh (multi-select form), Bitbucket REST API v2.0

---

### Task 1: Add `Project` type and `ListProjects` to Bitbucket provider

**Files:**
- Modify: `internal/provider/bitbucket.go`
- Create: `internal/provider/bitbucket_projects_test.go`

**Step 1: Write the failing test**

Create `internal/provider/bitbucket_projects_test.go`:

```go
package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsablic/codemium/internal/provider"
)

func TestBitbucketListProjects(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/workspaces/myws/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page++
		if page == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{"key": "PROJ1", "name": "Project One"},
					{"key": "PROJ2", "name": "Project Two"},
				},
				"next": "http://" + r.Host + "/2.0/workspaces/myws/projects?page=2",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{"key": "PROJ3", "name": "Project Three"},
				},
			})
		}
	}))
	defer server.Close()

	bb := provider.NewBitbucket("token", "user", server.URL)
	projects, err := bb.ListProjects(context.Background(), "myws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	if projects[0].Key != "PROJ1" || projects[0].Name != "Project One" {
		t.Errorf("unexpected first project: %+v", projects[0])
	}
	if projects[2].Key != "PROJ3" {
		t.Errorf("unexpected third project: %+v", projects[2])
	}
}

func TestBitbucketListProjectsAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{}})
	}))
	defer server.Close()

	bb := provider.NewBitbucket("token", "user", server.URL)
	bb.ListProjects(context.Background(), "ws")

	if len(gotAuth) < 6 || gotAuth[:6] != "Basic " {
		t.Errorf("expected Basic auth, got %s", gotAuth)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/ -run TestBitbucketListProject -v`
Expected: compilation error — `ListProjects` and `Project` don't exist yet.

**Step 3: Implement ListProjects**

Add to `internal/provider/bitbucket.go`:

1. Add the `Project` struct (exported, at the top with the other types):

```go
type Project struct {
	Key  string
	Name string
}
```

2. Add the `ListProjects` method. This reuses the existing auth logic from `fetchPage`. Add a generic `doGet` helper to avoid duplicating the auth setup, then use it in both `fetchPage` and `ListProjects`:

```go
func (b *Bitbucket) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if b.username != "" {
		req.SetBasicAuth(b.username, b.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}
	return b.client.Do(req)
}
```

Refactor `fetchPage` to use `doGet` (replace the `http.NewRequestWithContext` + auth block with `resp, err := b.doGet(ctx, pageURL)`).

Then add `ListProjects`:

```go
type bitbucketProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (b *Bitbucket) ListProjects(ctx context.Context, workspace string) ([]Project, error) {
	var all []Project
	nextURL := fmt.Sprintf("%s/2.0/workspaces/%s/projects?pagelen=100", b.baseURL, url.PathEscape(workspace))

	for nextURL != "" {
		resp, err := b.doGet(ctx, nextURL)
		if err != nil {
			return nil, fmt.Errorf("bitbucket projects request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bitbucket projects API returned status %d", resp.StatusCode)
		}

		var page struct {
			Values []bitbucketProject `json:"values"`
			Next   string             `json:"next"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return nil, fmt.Errorf("decode projects response: %w", err)
		}

		for _, p := range page.Values {
			all = append(all, Project{Key: p.Key, Name: p.Name})
		}
		nextURL = page.Next
	}

	return all, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/provider/ -v`
Expected: all tests pass including existing ones.

**Step 5: Commit**

```bash
git add internal/provider/bitbucket.go internal/provider/bitbucket_projects_test.go
git commit -m "feat: add ListProjects to Bitbucket provider"
```

---

### Task 2: Add project picker UI using huh

**Files:**
- Create: `internal/ui/picker.go`
- Modify: `go.mod` (add `charmbracelet/huh` dependency)

**Step 1: Add huh dependency**

Run: `go get github.com/charmbracelet/huh`

**Step 2: Write the picker function**

Create `internal/ui/picker.go`:

```go
package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/dsablic/codemium/internal/provider"
)

const selectAllKey = "__all__"

func PickProjects(projects []provider.Project) ([]string, error) {
	if len(projects) == 0 {
		return nil, nil
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Key < projects[j].Key
	})

	opts := make([]huh.Option[string], 0, len(projects)+1)
	opts = append(opts, huh.NewOption[string]("Select All", selectAllKey))
	for _, p := range projects {
		label := fmt.Sprintf("%s — %s", p.Key, p.Name)
		opts = append(opts, huh.NewOption[string](label, p.Key))
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select Bitbucket projects to analyze").
				Options(opts...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	for _, s := range selected {
		if s == selectAllKey {
			keys := make([]string, len(projects))
			for i, p := range projects {
				keys[i] = p.Key
			}
			return keys, nil
		}
	}

	return selected, nil
}
```

**Step 3: Verify it compiles**

Run: `go build ./internal/ui/`
Expected: compiles without error.

**Step 4: Run go mod tidy**

Run: `go mod tidy`

**Step 5: Commit**

```bash
git add internal/ui/picker.go go.mod go.sum
git commit -m "feat: add interactive project picker using huh"
```

---

### Task 3: Wire picker into analyze command

**Files:**
- Modify: `cmd/codemium/main.go`

**Step 1: Update runAnalyze**

In `cmd/codemium/main.go`, in the `runAnalyze` function, after the Bitbucket provider is created (around line 234) and before `ListRepos` is called (line 246), add the picker logic:

```go
	// Interactive project picker for Bitbucket
	if providerName == "bitbucket" && len(projects) == 0 && ui.IsTTY() {
		bb := prov.(*provider.Bitbucket)
		fmt.Fprintln(os.Stderr, "Fetching projects...")
		projectList, err := bb.ListProjects(ctx, workspace)
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		if len(projectList) > 0 {
			selected, err := ui.PickProjects(projectList)
			if err != nil {
				return fmt.Errorf("project picker: %w", err)
			}
			if len(selected) > 0 {
				projects = selected
			}
		}
	}
```

This goes between the `prov = provider.NewBitbucket(...)` block and the `fmt.Fprintln(os.Stderr, "Listing repositories...")` line.

**Step 2: Verify build**

Run: `go build ./cmd/codemium`
Expected: compiles without error.

**Step 3: Manual test**

Run: `./codemium analyze --provider bitbucket --workspace <your-workspace>`
Expected: shows "Fetching projects..." then the multi-select picker.

**Step 4: Commit**

```bash
git add cmd/codemium/main.go
git commit -m "feat: wire project picker into analyze command"
```

---

### Task 4: Final verification and cleanup

**Step 1: Run all tests**

Run: `go test ./... -short -count=1`
Expected: all pass.

**Step 2: Run vet**

Run: `go vet ./...`
Expected: clean.

**Step 3: Build**

Run: `go build ./cmd/codemium`
Expected: clean build.

**Step 4: Commit any remaining changes**

If `go mod tidy` changed anything:

```bash
git add go.mod go.sum
git commit -m "chore: tidy go.mod"
```
