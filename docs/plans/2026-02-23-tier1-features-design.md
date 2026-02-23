# Tier 1 Features Design

Date: 2026-02-23

## Overview

Three features to add to codemium's `analyze` command:

1. **Vendor/Generated File Filtering** — always-on, using `go-enry/go-enry/v2`
2. **License Detection** — per-repo SPDX identifier, using `go-enry/go-license-detector/v4`
3. **Code Churn / Hotspots** — opt-in via `--churn` flag, using provider REST APIs

## Feature 1: Vendor/Generated File Filtering

### Approach

Integrate `go-enry/go-enry/v2` into the analyzer's `filepath.Walk` loop. Before processing each file, call `enry.IsVendor(path)` and `enry.IsGenerated(name, content)`, skipping matches. Always on — no flag.

Replace the current hardcoded directory skip list (`vendor`, `node_modules`, `.git`, `.hg`) with enry's vendor detection. Keep scc's binary detection (`job.Binary`) as-is.

### Model Changes

Add a filtered file count to `RepoStats`:

```go
type RepoStats struct {
    // ...existing fields...
    FilteredFiles int64 `json:"filtered_files,omitempty"`
}
```

Single count (vendor + generated combined). The value is in cleaner numbers, not in a detailed breakdown.

### Output

Report summary gains a "Filtered Files" row. Per-repo table gains a column if any repo has filtered files.

## Feature 2: License Detection

### Approach

After cloning/downloading a repo, scan the directory with `go-enry/go-license-detector/v4/licensedb`. It returns SPDX identifiers with confidence scores. Take the top match above 0.85 confidence.

Runs inside the existing per-repo worker function, right after `Analyze()`, on the same cloned directory. Zero extra cloning.

### Model Changes

```go
type RepoStats struct {
    // ...existing fields...
    License string `json:"license,omitempty"`
}
```

### Output

License column added to the markdown Repositories table. JSON gets the field automatically.

## Feature 3: Code Churn / Hotspots

### Approach

Opt-in via `--churn` flag. Uses provider REST APIs (same pattern as `--ai-estimate`) to fetch file-level change data per commit. Runs as a separate pass after the main analysis, like AI estimation.

### Provider Interface

Extend or complement `CommitLister` with file-level diff stats:

- GitHub: `GET /repos/{owner}/{repo}/commits/{sha}` returns `files[]` with additions/deletions/filename
- Bitbucket: `GET /2.0/repositories/{workspace}/{repo}/diffstat/{spec}` returns per-file stats

### New Package

`internal/churn/` — orchestrates per-repo churn calculation.

### Data Model

```go
type FileChurn struct {
    Path       string  `json:"path"`
    Changes    int64   `json:"changes"`     // commits touching this file
    Additions  int64   `json:"additions"`
    Deletions  int64   `json:"deletions"`
    Complexity int64   `json:"complexity"`  // from analyzer, if available
    Hotspot    float64 `json:"hotspot"`     // churn * complexity normalized score
}

type ChurnStats struct {
    TotalCommits int64       `json:"total_commits"`
    TopFiles     []FileChurn `json:"top_files"`  // top 20 most churned
    Hotspots     []FileChurn `json:"hotspots"`   // top 10 high-churn + high-complexity
}
```

Attached to `RepoStats` as `Churn *ChurnStats` (optional, like `AIEstimate`).

### Hotspot Calculation

`hotspot_score = changes * complexity`. Files that change frequently AND have high complexity are the most actionable refactoring targets. Complexity comes from the analyzer's per-file data (requires tracking per-file complexity during analysis).

### Flags

- `--churn` — enables churn/hotspot analysis
- `--churn-limit N` — max commits to scan per repo (default 500)

### Output

Markdown: new "Code Churn" section with top churned files table and hotspots table.
JSON: `churn` field on each `RepoStats`, plus aggregate `churn` on `Report`.
