# Trends Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `codemium trends` command that analyzes repos at historical points in time via git history, producing time-series JSON that the existing `markdown` command auto-detects and renders.

**Architecture:** Full clone per repo, walk commit log once to find last commit before each target date, checkout and analyze sequentially. Worker pool parallelizes across repos. New `internal/history` package for date-to-commit resolution.

**Tech Stack:** go-git v5 (already a dependency), existing analyzer/worker/model packages.

---

### Task 1: Add TrendsReport and PeriodSnapshot to model

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/model/model_test.go`

**Step 1: Write the failing test**

Add to `internal/model/model_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestTrendsReportJSON -v`
Expected: FAIL — `model.TrendsReport` and `model.PeriodSnapshot` undefined.

**Step 3: Write minimal implementation**

Add to `internal/model/model.go`:

```go
// PeriodSnapshot holds stats for all repos at a single point in time.
type PeriodSnapshot struct {
	Period       string          `json:"period"`
	Repositories []RepoStats     `json:"repositories"`
	Totals       Stats           `json:"totals"`
	ByLanguage   []LanguageStats `json:"by_language"`
}

// TrendsReport is the top-level output for historical trends.
type TrendsReport struct {
	GeneratedAt  string           `json:"generated_at"`
	Provider     string           `json:"provider"`
	Workspace    string           `json:"workspace,omitempty"`
	Organization string           `json:"organization,omitempty"`
	Filters      Filters          `json:"filters"`
	Since        string           `json:"since"`
	Until        string           `json:"until"`
	Interval     string           `json:"interval"`
	Periods      []string         `json:"periods"`
	Snapshots    []PeriodSnapshot `json:"snapshots"`
	Errors       []RepoError      `json:"errors,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestTrendsReportJSON -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/model/model.go internal/model/model_test.go
git commit -m "feat: add TrendsReport and PeriodSnapshot model types"
```

---

### Task 2: Add Cloner.CloneFull() method

**Files:**
- Modify: `internal/analyzer/clone.go`
- Modify: `internal/analyzer/clone_test.go`

**Step 1: Write the failing test**

Add to `internal/analyzer/clone_test.go`:

```go
func TestCloneFullAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping clone test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cloner := analyzer.NewCloner("", "")

	repo, dir, cleanup, err := cloner.CloneFull(ctx, "https://github.com/kelseyhightower/nocode.git")
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	defer cleanup()

	// Verify we got a git.Repository handle
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}

	// Verify we can read the log (full history)
	logIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	count := 0
	logIter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	if count < 2 {
		t.Errorf("expected full history (>= 2 commits), got %d", count)
	}

	// Verify directory exists
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read cloned dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("cloned directory is empty")
	}
}
```

Note: test imports will need `git "github.com/go-git/go-git/v5"` and `"github.com/go-git/go-git/v5/plumbing/object"`.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/analyzer/ -run TestCloneFullAndCleanup -v`
Expected: FAIL — `CloneFull` method undefined.

**Step 3: Write minimal implementation**

Add to `internal/analyzer/clone.go`:

```go
import (
	git "github.com/go-git/go-git/v5"
)

// CloneFull clones the full repository (all history) at cloneURL into a
// temporary directory. It returns the go-git Repository handle, the directory
// path, a cleanup function, and any error. Used by the trends command to
// check out historical commits.
func (c *Cloner) CloneFull(ctx context.Context, cloneURL string) (repo *git.Repository, dir string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "codemium-*")
	if err != nil {
		return nil, "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanupFn := func() {
		os.RemoveAll(tmpDir)
	}

	opts := &git.CloneOptions{
		URL:  cloneURL,
		Tags: git.NoTags,
	}

	if c.token != "" {
		username := c.username
		if username == "" {
			username = "x-token-auth"
		}
		opts.Auth = &githttp.BasicAuth{
			Username: username,
			Password: c.token,
		}
	}

	r, err := git.PlainCloneContext(ctx, tmpDir, false, opts)
	if err != nil {
		cleanupFn()
		return nil, "", nil, fmt.Errorf("git clone: %w", err)
	}

	return r, tmpDir, cleanupFn, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/analyzer/ -run TestCloneFullAndCleanup -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/analyzer/clone.go internal/analyzer/clone_test.go
git commit -m "feat: add CloneFull method for full-history cloning"
```

---

### Task 3: Create internal/history package — FindCommits and date generation

**Files:**
- Create: `internal/history/history.go`
- Create: `internal/history/history_test.go`

**Step 1: Write the failing test**

Create `internal/history/history_test.go`:

```go
package history_test

import (
	"testing"
	"time"

	"github.com/dsablic/codemium/internal/history"
)

func TestGenerateDatesMonthly(t *testing.T) {
	dates := history.GenerateDates("2025-01", "2025-03", "monthly")
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}
	// Each date should be end-of-month (last day, 23:59:59 UTC)
	if dates[0].Month() != time.January || dates[0].Year() != 2025 {
		t.Errorf("first date wrong: %v", dates[0])
	}
	if dates[2].Month() != time.March || dates[2].Year() != 2025 {
		t.Errorf("last date wrong: %v", dates[2])
	}
}

func TestGenerateDatesWeekly(t *testing.T) {
	dates := history.GenerateDates("2025-01-01", "2025-01-22", "weekly")
	if len(dates) != 4 {
		t.Fatalf("expected 4 dates, got %d: %v", len(dates), dates)
	}
}

func TestFormatPeriodMonthly(t *testing.T) {
	d := time.Date(2025, 3, 31, 23, 59, 59, 0, time.UTC)
	got := history.FormatPeriod(d, "monthly")
	if got != "2025-03" {
		t.Errorf("expected 2025-03, got %s", got)
	}
}

func TestFormatPeriodWeekly(t *testing.T) {
	d := time.Date(2025, 1, 15, 23, 59, 59, 0, time.UTC)
	got := history.FormatPeriod(d, "weekly")
	if got != "2025-01-15" {
		t.Errorf("expected 2025-01-15, got %s", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/history/ -v`
Expected: FAIL — package doesn't exist.

**Step 3: Write minimal implementation**

Create `internal/history/history.go`:

```go
package history

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GenerateDates produces target dates from since to until at the given interval.
// For monthly: since/until are "YYYY-MM", dates are end-of-month.
// For weekly: since/until are "YYYY-MM-DD", dates are 7 days apart.
func GenerateDates(since, until, interval string) []time.Time {
	var dates []time.Time

	switch interval {
	case "weekly":
		start, _ := time.Parse("2006-01-02", since)
		end, _ := time.Parse("2006-01-02", until)
		end = endOfDay(end)
		for d := endOfDay(start); !d.After(end); d = d.AddDate(0, 0, 7) {
			dates = append(dates, d)
		}
	default: // monthly
		start, _ := time.Parse("2006-01", since)
		end, _ := time.Parse("2006-01", until)
		for d := start; !d.After(end); d = d.AddDate(0, 1, 0) {
			dates = append(dates, endOfMonth(d))
		}
	}

	return dates
}

// FormatPeriod formats a date as a period label.
func FormatPeriod(d time.Time, interval string) string {
	if interval == "weekly" {
		return d.Format("2006-01-02")
	}
	return d.Format("2006-01")
}

// FindCommits walks the default branch log and returns the last commit
// at or before each target date. Dates with no prior commits are omitted.
func FindCommits(repo *git.Repository, dates []time.Time) (map[time.Time]plumbing.Hash, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, err
	}

	logIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}

	// Collect all commits (newest first)
	var commits []*object.Commit
	err = logIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make(map[time.Time]plumbing.Hash, len(dates))
	for _, target := range dates {
		for _, c := range commits {
			if !c.Author.When.After(target) {
				result[target] = c.Hash
				break
			}
		}
	}

	return result, nil
}

func endOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 23, 59, 59, 0, time.UTC)
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/history/ -v`
Expected: PASS

**Step 5: Write FindCommits test with a real git repo**

Add to `internal/history/history_test.go`:

```go
func TestFindCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git test in short mode")
	}

	// Clone a small public repo with known history
	dir, err := os.MkdirTemp("", "history-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL: "https://github.com/kelseyhightower/nocode.git",
	})
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	// Use dates far in the future (should find HEAD) and far in the past (should be omitted)
	dates := []time.Time{
		time.Date(2000, 1, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
	}

	result, err := history.FindCommits(repo, dates)
	if err != nil {
		t.Fatalf("FindCommits failed: %v", err)
	}

	// 2000 should be omitted (repo didn't exist)
	if _, ok := result[dates[0]]; ok {
		t.Error("expected no commit for year 2000")
	}

	// 2099 should have a commit
	if _, ok := result[dates[1]]; !ok {
		t.Error("expected a commit for year 2099")
	}
}
```

Note: test needs imports for `"context"`, `"os"`, `git "github.com/go-git/go-git/v5"`.

**Step 6: Run test**

Run: `go test ./internal/history/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/history/
git commit -m "feat: add history package with date generation and commit resolution"
```

---

### Task 4: Add checkout helper to Cloner

**Files:**
- Modify: `internal/analyzer/clone.go`
- Modify: `internal/analyzer/clone_test.go`

**Step 1: Write the failing test**

Add to `internal/analyzer/clone_test.go`:

```go
func TestCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cloner := analyzer.NewCloner("", "")

	repo, dir, cleanup, err := cloner.CloneFull(ctx, "https://github.com/kelseyhightower/nocode.git")
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	defer cleanup()

	// Get HEAD hash
	ref, err := repo.Head()
	if err != nil {
		t.Fatalf("head failed: %v", err)
	}

	// Checkout should succeed for HEAD
	if err := analyzer.Checkout(repo, dir, ref.Hash()); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/analyzer/ -run TestCheckout -v`
Expected: FAIL — `analyzer.Checkout` undefined.

**Step 3: Write minimal implementation**

Add to `internal/analyzer/clone.go`:

```go
import (
	"github.com/go-git/go-git/v5/plumbing"
)

// Checkout checks out a specific commit in the given repository worktree.
func Checkout(repo *git.Repository, dir string, hash plumbing.Hash) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}
	return wt.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/analyzer/ -run TestCheckout -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/analyzer/clone.go internal/analyzer/clone_test.go
git commit -m "feat: add Checkout helper for switching to historical commits"
```

---

### Task 5: Add WriteTrendsJSON to output package

**Files:**
- Modify: `internal/output/json.go`
- Modify: `internal/output/output_test.go`

**Step 1: Write the failing test**

Add to `internal/output/output_test.go`:

```go
func TestWriteTrendsJSON(t *testing.T) {
	report := model.TrendsReport{
		GeneratedAt:  "2026-02-19T12:00:00Z",
		Provider:     "github",
		Organization: "myorg",
		Since:        "2025-01",
		Until:        "2025-02",
		Interval:     "monthly",
		Periods:      []string{"2025-01", "2025-02"},
		Snapshots: []model.PeriodSnapshot{
			{
				Period: "2025-01",
				Totals: model.Stats{Repos: 1, Code: 1000},
			},
			{
				Period: "2025-02",
				Totals: model.Stats{Repos: 1, Code: 1200},
			},
		},
	}

	var buf bytes.Buffer
	if err := output.WriteTrendsJSON(&buf, report); err != nil {
		t.Fatalf("failed to write trends JSON: %v", err)
	}

	var decoded model.TrendsReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded.Snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(decoded.Snapshots))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -run TestWriteTrendsJSON -v`
Expected: FAIL — `output.WriteTrendsJSON` undefined.

**Step 3: Write minimal implementation**

Add to `internal/output/json.go`:

```go
// WriteTrendsJSON writes the trends report as pretty-printed JSON to w.
func WriteTrendsJSON(w io.Writer, report model.TrendsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/output/ -run TestWriteTrendsJSON -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/output/json.go internal/output/output_test.go
git commit -m "feat: add WriteTrendsJSON output function"
```

---

### Task 6: Add trends markdown rendering with auto-detection

**Files:**
- Modify: `internal/output/markdown.go`
- Modify: `internal/output/output_test.go`

**Step 1: Write the failing test**

Add to `internal/output/output_test.go`:

```go
func TestWriteTrendsMarkdown(t *testing.T) {
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
					{Repository: "api", Totals: model.Stats{Code: 1000}},
				},
				Totals:     model.Stats{Repos: 1, Code: 1000, Files: 10},
				ByLanguage: []model.LanguageStats{{Name: "Go", Code: 1000}},
			},
			{
				Period: "2025-02",
				Repositories: []model.RepoStats{
					{Repository: "api", Totals: model.Stats{Code: 1200}},
				},
				Totals:     model.Stats{Repos: 1, Code: 1200, Files: 12},
				ByLanguage: []model.LanguageStats{{Name: "Go", Code: 1200}},
			},
			{
				Period: "2025-03",
				Repositories: []model.RepoStats{
					{Repository: "api", Totals: model.Stats{Code: 1500}},
				},
				Totals:     model.Stats{Repos: 1, Code: 1500, Files: 15},
				ByLanguage: []model.LanguageStats{{Name: "Go", Code: 1500}},
			},
		},
	}

	var buf bytes.Buffer
	if err := output.WriteTrendsMarkdown(&buf, report); err != nil {
		t.Fatalf("failed to write trends markdown: %v", err)
	}

	md := buf.String()

	// Should contain period columns
	if !strings.Contains(md, "2025-01") {
		t.Error("markdown should contain period 2025-01")
	}
	if !strings.Contains(md, "2025-03") {
		t.Error("markdown should contain period 2025-03")
	}
	// Should contain repo name
	if !strings.Contains(md, "api") {
		t.Error("markdown should contain repo name")
	}
	// Should contain delta info (e.g. +200 or +300)
	if !strings.Contains(md, "+") {
		t.Error("markdown should contain delta indicators")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -run TestWriteTrendsMarkdown -v`
Expected: FAIL — `output.WriteTrendsMarkdown` undefined.

**Step 3: Write implementation**

Add to `internal/output/markdown.go`:

```go
// WriteTrendsMarkdown writes the trends report as GitHub-flavored markdown.
func WriteTrendsMarkdown(w io.Writer, report model.TrendsReport) error {
	fmt.Fprintf(w, "# Code Trends Report\n\n")
	fmt.Fprintf(w, "**Provider:** %s\n", report.Provider)
	if report.Workspace != "" {
		fmt.Fprintf(w, "**Workspace:** %s\n", report.Workspace)
	}
	if report.Organization != "" {
		fmt.Fprintf(w, "**Organization:** %s\n", report.Organization)
	}
	fmt.Fprintf(w, "**Period:** %s to %s (%s)\n", report.Since, report.Until, report.Interval)
	fmt.Fprintf(w, "**Generated:** %s\n\n", report.GeneratedAt)

	// Summary table: one row per period
	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Period | Files | Code | Comments | Blanks | Complexity | Code Delta |\n")
	fmt.Fprintf(w, "|--------|------:|-----:|---------:|-------:|-----------:|-----------:|\n")
	var prevCode int64
	for _, snap := range report.Snapshots {
		delta := ""
		if prevCode > 0 {
			diff := snap.Totals.Code - prevCode
			if diff >= 0 {
				delta = fmt.Sprintf("+%d", diff)
			} else {
				delta = fmt.Sprintf("%d", diff)
			}
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %s |\n",
			snap.Period, snap.Totals.Files, snap.Totals.Code,
			snap.Totals.Comments, snap.Totals.Blanks, snap.Totals.Complexity, delta)
		prevCode = snap.Totals.Code
	}
	fmt.Fprintln(w)

	// Language breakdown across time
	fmt.Fprintf(w, "## Languages Over Time\n\n")
	fmt.Fprintf(w, "| Language |")
	for _, p := range report.Periods {
		fmt.Fprintf(w, " %s |", p)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "|----------|")
	for range report.Periods {
		fmt.Fprintf(w, "-----:|")
	}
	fmt.Fprintln(w)

	// Collect all language names across all snapshots
	langSet := map[string]bool{}
	for _, snap := range report.Snapshots {
		for _, lang := range snap.ByLanguage {
			langSet[lang.Name] = true
		}
	}
	// Sort language names for deterministic output
	var langNames []string
	for name := range langSet {
		langNames = append(langNames, name)
	}
	sort.Strings(langNames)

	for _, name := range langNames {
		fmt.Fprintf(w, "| %s |", name)
		for _, snap := range report.Snapshots {
			code := int64(0)
			for _, lang := range snap.ByLanguage {
				if lang.Name == name {
					code = lang.Code
					break
				}
			}
			fmt.Fprintf(w, " %d |", code)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)

	// Per-repo code over time
	fmt.Fprintf(w, "## Repositories Over Time\n\n")
	fmt.Fprintf(w, "| Repository |")
	for _, p := range report.Periods {
		fmt.Fprintf(w, " %s |", p)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "|------------|")
	for range report.Periods {
		fmt.Fprintf(w, "-----:|")
	}
	fmt.Fprintln(w)

	// Collect all repo names
	repoSet := map[string]bool{}
	for _, snap := range report.Snapshots {
		for _, repo := range snap.Repositories {
			repoSet[repo.Repository] = true
		}
	}
	var repoNames []string
	for name := range repoSet {
		repoNames = append(repoNames, name)
	}
	sort.Strings(repoNames)

	for _, name := range repoNames {
		fmt.Fprintf(w, "| %s |", name)
		for _, snap := range report.Snapshots {
			code := int64(0)
			for _, repo := range snap.Repositories {
				if repo.Repository == name {
					code = repo.Totals.Code
					break
				}
			}
			fmt.Fprintf(w, " %d |", code)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)

	// Errors
	if len(report.Errors) > 0 {
		fmt.Fprintf(w, "## Errors\n\n")
		for _, e := range report.Errors {
			fmt.Fprintf(w, "- **%s**: %s\n", e.Repository, e.Error)
		}
		fmt.Fprintln(w)
	}

	return nil
}
```

Note: needs `"sort"` added to imports.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/output/ -run TestWriteTrendsMarkdown -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/output/markdown.go internal/output/output_test.go
git commit -m "feat: add WriteTrendsMarkdown for time-series reports"
```

---

### Task 7: Update markdown command to auto-detect report type

**Files:**
- Modify: `cmd/codemium/main.go`

**Step 1: Update runMarkdown to auto-detect**

Replace the `runMarkdown` function in `cmd/codemium/main.go`:

```go
func runMarkdown(cmd *cobra.Command, args []string) error {
	var r io.Reader = os.Stdin
	if len(args) == 1 {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// Auto-detect report type by trying TrendsReport first
	var trends model.TrendsReport
	if err := json.Unmarshal(data, &trends); err == nil && len(trends.Snapshots) > 0 {
		return output.WriteTrendsMarkdown(os.Stdout, trends)
	}

	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse JSON report: %w", err)
	}

	return output.WriteMarkdown(os.Stdout, report)
}
```

**Step 2: Verify build**

Run: `go build ./cmd/codemium`
Expected: builds without errors.

**Step 3: Commit**

```bash
git add cmd/codemium/main.go
git commit -m "feat: auto-detect trends vs standard report in markdown command"
```

---

### Task 8: Wire up the trends command

**Files:**
- Modify: `cmd/codemium/main.go`

**Step 1: Add newTrendsCmd function and register it**

Add to `cmd/codemium/main.go`, and add `root.AddCommand(newTrendsCmd())` in `main()`:

```go
func newTrendsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Analyze repository trends over time using git history",
		RunE:  runTrends,
	}

	cmd.Flags().String("provider", "", "Provider (bitbucket, github)")
	cmd.Flags().String("workspace", "", "Bitbucket workspace slug")
	cmd.Flags().String("org", "", "GitHub organization")
	cmd.Flags().String("since", "", "Start period (YYYY-MM for monthly, YYYY-MM-DD for weekly)")
	cmd.Flags().String("until", "", "End period (YYYY-MM for monthly, YYYY-MM-DD for weekly)")
	cmd.Flags().String("interval", "monthly", "Interval: monthly or weekly")
	cmd.Flags().StringSlice("repos", nil, "Filter to specific repo names")
	cmd.Flags().StringSlice("exclude", nil, "Exclude specific repos")
	cmd.Flags().Bool("include-archived", false, "Include archived repos")
	cmd.Flags().Bool("include-forks", false, "Include forked repos")
	cmd.Flags().Int("concurrency", 5, "Number of parallel workers")
	cmd.Flags().String("output", "", "Write JSON to file (default: stdout)")

	cmd.MarkFlagRequired("provider")
	cmd.MarkFlagRequired("since")
	cmd.MarkFlagRequired("until")

	return cmd
}

func runTrends(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	providerName, _ := cmd.Flags().GetString("provider")
	workspace, _ := cmd.Flags().GetString("workspace")
	org, _ := cmd.Flags().GetString("org")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	interval, _ := cmd.Flags().GetString("interval")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	includeForks, _ := cmd.Flags().GetBool("include-forks")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	outputPath, _ := cmd.Flags().GetString("output")

	if interval != "monthly" && interval != "weekly" {
		return fmt.Errorf("--interval must be 'monthly' or 'weekly'")
	}

	// Load credentials
	store := auth.NewFileStore(auth.DefaultStorePath())
	cred, err := store.LoadWithEnv(providerName)
	if err != nil {
		return fmt.Errorf("not authenticated with %s — run 'codemium auth login --provider %s' first", providerName, providerName)
	}

	if cred.Expired() && cred.RefreshToken != "" {
		clientID := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_ID")
		clientSecret := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_SECRET")
		bb := &auth.BitbucketOAuth{ClientID: clientID, ClientSecret: clientSecret}
		cred, err = bb.RefreshToken(ctx, cred.RefreshToken)
		if err != nil {
			return fmt.Errorf("token refresh failed: %w", err)
		}
		store.Save(providerName, cred)
	}

	var prov provider.Provider
	switch providerName {
	case "bitbucket":
		if workspace == "" {
			return fmt.Errorf("--workspace is required for bitbucket")
		}
		prov = provider.NewBitbucket(cred.AccessToken, cred.Username, "")
	case "github":
		if org == "" {
			return fmt.Errorf("--org is required for github")
		}
		prov = provider.NewGitHub(cred.AccessToken, "")
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}

	// List repos
	fmt.Fprintln(os.Stderr, "Listing repositories...")
	repoList, err := prov.ListRepos(ctx, provider.ListOpts{
		Workspace:       workspace,
		Organization:    org,
		Repos:           repos,
		Exclude:         exclude,
		IncludeArchived: includeArchived,
		IncludeForks:    includeForks,
	})
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}
	if len(repoList) == 0 {
		return fmt.Errorf("no repositories found")
	}

	// Generate target dates
	dates := history.GenerateDates(since, until, interval)
	if len(dates) == 0 {
		return fmt.Errorf("no periods generated for --since %s --until %s --interval %s", since, until, interval)
	}

	periods := make([]string, len(dates))
	for i, d := range dates {
		periods[i] = history.FormatPeriod(d, interval)
	}

	fmt.Fprintf(os.Stderr, "Found %d repositories, analyzing %d %s periods\n", len(repoList), len(dates), interval)

	// Set up progress
	totalWork := len(repoList)
	useTUI := ui.IsTTY()
	var program *tea.Program
	if useTUI {
		program = ui.RunTUI(totalWork)
		go func() { program.Run() }()
	}

	// Process repos — each worker does full clone + all snapshots
	cloner := analyzer.NewCloner(cred.AccessToken, cred.Username)
	codeAnalyzer := analyzer.New()

	type repoSnapshots struct {
		Repo      model.Repo
		Snapshots map[string]*model.RepoStats // period -> stats
		Err       error
	}

	progressFn := func(completed, total int, repo model.Repo) {
		if useTUI && program != nil {
			program.Send(ui.ProgressMsg{
				Completed: completed,
				Total:     total,
				RepoName:  repo.Slug,
			})
		} else {
			fmt.Fprintf(os.Stderr, "[%d/%d] Analyzed %s (%d snapshots)\n", completed, total, repo.Slug, len(dates))
		}
	}

	results := worker.RunWithProgress(ctx, repoList, concurrency, func(ctx context.Context, repo model.Repo) (*model.RepoStats, error) {
		// Full clone
		gitRepo, dir, cleanup, err := cloner.CloneFull(ctx, repo.CloneURL)
		if err != nil {
			return nil, err
		}
		defer cleanup()

		// Find commits for each date
		commitMap, err := history.FindCommits(gitRepo, dates)
		if err != nil {
			return nil, fmt.Errorf("find commits: %w", err)
		}

		// Analyze each snapshot — we encode the per-snapshot results into
		// a special RepoStats with JSON-encoded snapshot data in a field.
		// Actually, we need a different approach since the worker returns
		// a single RepoStats per repo...
		return nil, nil
	}, progressFn)
```

Wait — the current worker pool returns `*model.RepoStats` per repo, but for trends we need `map[period]*RepoStats` per repo. We need a different worker type.

**Step 1 (revised): Add a generic worker function to worker package**

Add to `internal/worker/pool.go`:

```go
// TrendsResult holds the outcome of processing a single repository across time periods.
type TrendsResult struct {
	Repo      model.Repo
	Snapshots map[string]*model.RepoStats // period label -> stats
	Err       error
}

// TrendsProcessFunc processes a single repository and returns stats per period.
type TrendsProcessFunc func(ctx context.Context, repo model.Repo) (map[string]*model.RepoStats, error)

// RunTrends processes repos concurrently, where each repo produces multiple period snapshots.
func RunTrends(ctx context.Context, repos []model.Repo, concurrency int, process TrendsProcessFunc, onProgress ProgressFunc) []TrendsResult {
	if concurrency < 1 {
		concurrency = 1
	}

	var (
		mu        sync.Mutex
		results   []TrendsResult
		completed int
	)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, repo := range repos {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(r model.Repo) {
			defer wg.Done()
			defer func() { <-sem }()

			snapshots, err := process(ctx, r)

			mu.Lock()
			results = append(results, TrendsResult{Repo: r, Snapshots: snapshots, Err: err})
			completed++
			c := completed
			mu.Unlock()

			if onProgress != nil {
				onProgress(c, len(repos), r)
			}
		}(repo)
	}

	wg.Wait()
	return results
}
```

**Step 2: Write runTrends using RunTrends**

The `runTrends` function in `cmd/codemium/main.go` (complete version):

```go
func runTrends(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer cancel()

	providerName, _ := cmd.Flags().GetString("provider")
	workspace, _ := cmd.Flags().GetString("workspace")
	org, _ := cmd.Flags().GetString("org")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	interval, _ := cmd.Flags().GetString("interval")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	includeArchived, _ := cmd.Flags().GetBool("include-archived")
	includeForks, _ := cmd.Flags().GetBool("include-forks")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	outputPath, _ := cmd.Flags().GetString("output")

	if interval != "monthly" && interval != "weekly" {
		return fmt.Errorf("--interval must be 'monthly' or 'weekly'")
	}

	store := auth.NewFileStore(auth.DefaultStorePath())
	cred, err := store.LoadWithEnv(providerName)
	if err != nil {
		return fmt.Errorf("not authenticated with %s — run 'codemium auth login --provider %s' first", providerName, providerName)
	}

	if cred.Expired() && cred.RefreshToken != "" {
		clientID := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_ID")
		clientSecret := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_SECRET")
		bb := &auth.BitbucketOAuth{ClientID: clientID, ClientSecret: clientSecret}
		cred, err = bb.RefreshToken(ctx, cred.RefreshToken)
		if err != nil {
			return fmt.Errorf("token refresh failed: %w", err)
		}
		store.Save(providerName, cred)
	}

	var prov provider.Provider
	switch providerName {
	case "bitbucket":
		if workspace == "" {
			return fmt.Errorf("--workspace is required for bitbucket")
		}
		prov = provider.NewBitbucket(cred.AccessToken, cred.Username, "")
	case "github":
		if org == "" {
			return fmt.Errorf("--org is required for github")
		}
		prov = provider.NewGitHub(cred.AccessToken, "")
	default:
		return fmt.Errorf("unsupported provider: %s", providerName)
	}

	fmt.Fprintln(os.Stderr, "Listing repositories...")
	repoList, err := prov.ListRepos(ctx, provider.ListOpts{
		Workspace:       workspace,
		Organization:    org,
		Repos:           repos,
		Exclude:         exclude,
		IncludeArchived: includeArchived,
		IncludeForks:    includeForks,
	})
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}
	if len(repoList) == 0 {
		return fmt.Errorf("no repositories found")
	}

	dates := history.GenerateDates(since, until, interval)
	if len(dates) == 0 {
		return fmt.Errorf("no periods generated for --since %s --until %s --interval %s", since, until, interval)
	}

	periods := make([]string, len(dates))
	for i, d := range dates {
		periods[i] = history.FormatPeriod(d, interval)
	}

	fmt.Fprintf(os.Stderr, "Found %d repositories, analyzing %d %s periods\n", len(repoList), len(dates), interval)

	useTUI := ui.IsTTY()
	var program *tea.Program
	if useTUI {
		program = ui.RunTUI(len(repoList))
		go func() { program.Run() }()
	}

	cloner := analyzer.NewCloner(cred.AccessToken, cred.Username)
	codeAnalyzer := analyzer.New()

	progressFn := func(completed, total int, repo model.Repo) {
		if useTUI && program != nil {
			program.Send(ui.ProgressMsg{
				Completed: completed,
				Total:     total,
				RepoName:  repo.Slug,
			})
		} else {
			fmt.Fprintf(os.Stderr, "[%d/%d] Analyzed %s\n", completed, total, repo.Slug)
		}
	}

	results := worker.RunTrends(ctx, repoList, concurrency, func(ctx context.Context, repo model.Repo) (map[string]*model.RepoStats, error) {
		gitRepo, dir, cleanup, err := cloner.CloneFull(ctx, repo.CloneURL)
		if err != nil {
			return nil, err
		}
		defer cleanup()

		commitMap, err := history.FindCommits(gitRepo, dates)
		if err != nil {
			return nil, fmt.Errorf("find commits: %w", err)
		}

		snapshots := make(map[string]*model.RepoStats, len(dates))
		for i, date := range dates {
			hash, ok := commitMap[date]
			if !ok {
				continue
			}

			if err := analyzer.Checkout(gitRepo, dir, hash); err != nil {
				continue
			}

			stats, err := codeAnalyzer.Analyze(ctx, dir)
			if err != nil {
				continue
			}

			stats.Repository = repo.Slug
			stats.Project = repo.Project
			stats.Provider = repo.Provider
			stats.URL = repo.URL
			snapshots[periods[i]] = stats
		}

		return snapshots, nil
	}, progressFn)

	if useTUI && program != nil {
		program.Send(ui.DoneMsg{})
		time.Sleep(100 * time.Millisecond)
		program.Quit()
	}

	report := buildTrendsReport(providerName, workspace, org, since, until, interval, periods, repos, exclude, results)

	var jsonWriter io.Writer = os.Stdout
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		jsonWriter = f
	}

	return output.WriteTrendsJSON(jsonWriter, report)
}

func buildTrendsReport(providerName, workspace, org, since, until, interval string, periods, repos, exclude []string, results []worker.TrendsResult) model.TrendsReport {
	report := model.TrendsReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Provider:     providerName,
		Workspace:    workspace,
		Organization: org,
		Filters: model.Filters{
			Repos:   repos,
			Exclude: exclude,
		},
		Since:    since,
		Until:    until,
		Interval: interval,
		Periods:  periods,
	}

	// Build snapshots: one PeriodSnapshot per period
	snapshotMap := make(map[string]*model.PeriodSnapshot, len(periods))
	for _, p := range periods {
		snapshotMap[p] = &model.PeriodSnapshot{Period: p}
	}

	for _, r := range results {
		if r.Err != nil {
			report.Errors = append(report.Errors, model.RepoError{
				Repository: r.Repo.Slug,
				Error:      r.Err.Error(),
			})
			continue
		}

		for period, stats := range r.Snapshots {
			snap := snapshotMap[period]
			snap.Repositories = append(snap.Repositories, *stats)
			snap.Totals.Repos++
			snap.Totals.Files += stats.Totals.Files
			snap.Totals.Lines += stats.Totals.Lines
			snap.Totals.Code += stats.Totals.Code
			snap.Totals.Comments += stats.Totals.Comments
			snap.Totals.Blanks += stats.Totals.Blanks
			snap.Totals.Complexity += stats.Totals.Complexity

			// Aggregate by language
			langMap := map[string]*model.LanguageStats{}
			for _, existing := range snap.ByLanguage {
				copy := existing
				langMap[existing.Name] = &copy
			}
			for _, lang := range stats.Languages {
				lt, ok := langMap[lang.Name]
				if !ok {
					lt = &model.LanguageStats{Name: lang.Name}
					langMap[lang.Name] = lt
				}
				lt.Files += lang.Files
				lt.Lines += lang.Lines
				lt.Code += lang.Code
				lt.Comments += lang.Comments
				lt.Blanks += lang.Blanks
				lt.Complexity += lang.Complexity
			}
			snap.ByLanguage = nil
			for _, lt := range langMap {
				snap.ByLanguage = append(snap.ByLanguage, *lt)
			}
			sort.Slice(snap.ByLanguage, func(i, j int) bool {
				return snap.ByLanguage[i].Code > snap.ByLanguage[j].Code
			})
		}
	}

	for _, p := range periods {
		report.Snapshots = append(report.Snapshots, *snapshotMap[p])
	}

	return report
}
```

Note: `cmd/codemium/main.go` will need the `history` import: `"github.com/dsablic/codemium/internal/history"`.

**Step 3: Verify build**

Run: `go build ./cmd/codemium`
Expected: builds without errors.

**Step 4: Commit**

```bash
git add internal/worker/pool.go cmd/codemium/main.go
git commit -m "feat: add codemium trends command"
```

---

### Task 9: Update docs

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Add trends section to README**

Add a "Trends" section under Usage:

```markdown
### Analyze trends over time

\```bash
# Monthly trends for the past year
codemium trends --provider github --org myorg --since 2025-03 --until 2026-02

# Weekly trends
codemium trends --provider github --org myorg --since 2025-01-01 --until 2025-03-01 --interval weekly

# Output to file, then convert to markdown
codemium trends --provider github --org myorg --since 2025-01 --until 2025-12 --output trends.json
codemium markdown trends.json > trends.md
\```
```

**Step 2: Update CLAUDE.md**

Add `history/` to the project structure section:

```
  history/
    history.go         Date generation and git commit resolution for trends
```

And add `trends` command info to the CLI section.

**Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: add trends command usage and update project structure"
```

---

### Task 10: Run full test suite

**Step 1: Run all tests**

Run: `go test ./... -short -v`
Expected: All tests pass.

**Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues.

**Step 3: Build**

Run: `go build ./cmd/codemium`
Expected: Clean build.
