// internal/health/details.go
package health

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

const statsConcurrency = 5

// Window labels for bucketing commits by age.
const (
	Window0to6   = "0-6mo"
	Window6to12  = "6-12mo"
	Window12Plus = "12mo+"
)

// AnalyzeDetails performs deep health analysis on a repo's commits.
// It returns the details, a list of partial error messages (e.g. per-commit stat failures), and a fatal error.
func AnalyzeDetails(ctx context.Context, lister provider.CommitLister, repo model.Repo, commits []provider.CommitInfo, now time.Time) (*model.RepoHealthDetails, []string, error) {
	if len(commits) == 0 {
		return &model.RepoHealthDetails{
			AuthorsByWindow: map[string]int{},
			ChurnByWindow:   map[string]model.WindowChurnStats{},
		}, nil, nil
	}

	sixMoAgo := now.AddDate(0, -6, 0)
	twelveMoAgo := now.AddDate(0, -12, 0)

	// Bucket commits by window
	authorSets := map[string]map[string]bool{
		Window0to6:   {},
		Window6to12:  {},
		Window12Plus: {},
	}
	churn := map[string]*model.WindowChurnStats{
		Window0to6:   {},
		Window6to12:  {},
		Window12Plus: {},
	}
	authorCommitCounts := map[string]int{}

	for _, c := range commits {
		window := commitWindow(c.Date, sixMoAgo, twelveMoAgo)
		author := normalizeAuthor(c.Author)
		authorSets[window][author] = true
		authorCommitCounts[author]++
		churn[window].Commits++
	}

	// Fetch commit stats concurrently
	type statResult struct {
		idx       int
		additions int64
		deletions int64
	}
	results := make([]statResult, len(commits))
	sem := make(chan struct{}, statsConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var partialErrors []string

	for i, c := range commits {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, hash string) {
			defer wg.Done()
			defer func() { <-sem }()

			adds, dels, err := lister.CommitStats(ctx, repo, hash)
			if err != nil {
				mu.Lock()
				partialErrors = append(partialErrors, fmt.Sprintf("CommitStats %s: %v", hash, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			results[idx] = statResult{idx: idx, additions: adds, deletions: dels}
			mu.Unlock()
		}(i, c.Hash)
	}
	wg.Wait()

	// We don't fail on stat errors — just use what we got
	for i, c := range commits {
		window := commitWindow(c.Date, sixMoAgo, twelveMoAgo)
		cs := churn[window]
		cs.Additions += results[i].additions
		cs.Deletions += results[i].deletions
	}

	// Compute net churn
	for _, cs := range churn {
		cs.NetChurn = cs.Additions - cs.Deletions
	}

	// Compute bus factor: % of commits from top author
	var busFactor float64
	if len(commits) > 0 {
		maxCommits := 0
		for _, count := range authorCommitCounts {
			if count > maxCommits {
				maxCommits = count
			}
		}
		busFactor = float64(maxCommits) / float64(len(commits)) * 100
	}

	// Compute velocity trend: commits in 0-6mo / commits in 6-12mo
	var velocityTrend float64
	recent := churn[Window0to6].Commits
	previous := churn[Window6to12].Commits
	if previous > 0 {
		velocityTrend = float64(recent) / float64(previous)
	}

	// Build authors by window
	authorsByWindow := map[string]int{}
	for window, authors := range authorSets {
		if len(authors) > 0 {
			authorsByWindow[window] = len(authors)
		}
	}

	// Build churn by window
	churnByWindow := map[string]model.WindowChurnStats{}
	for window, cs := range churn {
		if cs.Commits > 0 {
			churnByWindow[window] = *cs
		}
	}

	return &model.RepoHealthDetails{
		AuthorsByWindow: authorsByWindow,
		ChurnByWindow:   churnByWindow,
		BusFactor:       busFactor,
		VelocityTrend:   velocityTrend,
	}, partialErrors, nil
}

func commitWindow(date time.Time, sixMoAgo, twelveMoAgo time.Time) string {
	switch {
	case date.After(sixMoAgo) || date.Equal(sixMoAgo):
		return Window0to6
	case date.After(twelveMoAgo) || date.Equal(twelveMoAgo):
		return Window6to12
	default:
		return Window12Plus
	}
}

func normalizeAuthor(author string) string {
	// Extract email from "Name <email>" format for deduplication
	if idx := strings.Index(author, "<"); idx >= 0 {
		if end := strings.Index(author[idx:], ">"); end >= 0 {
			return strings.ToLower(strings.TrimSpace(author[idx+1 : idx+end]))
		}
	}
	return strings.ToLower(strings.TrimSpace(author))
}
