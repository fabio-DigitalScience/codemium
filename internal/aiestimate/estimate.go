package aiestimate

import (
	"context"
	"fmt"
	"sync"

	"github.com/dsablic/codemium/internal/aidetect"
	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

const statsConcurrency = 10

// Estimate computes AI attribution metrics for a single repo.
// It returns the estimate, a list of partial error messages (per-commit stat failures), and a fatal error.
func Estimate(ctx context.Context, cl provider.CommitLister, repo model.Repo, commitLimit int) (*model.AIEstimate, []string, error) {
	commits, err := cl.ListCommits(ctx, repo, commitLimit)
	if err != nil {
		return nil, nil, err
	}

	est := &model.AIEstimate{
		TotalCommits: int64(len(commits)),
	}

	type flaggedCommit struct {
		info    provider.CommitInfo
		signals []model.AISignal
	}

	var flagged []flaggedCommit
	for _, c := range commits {
		signals := aidetect.Detect(c.Author, c.Message)
		if len(signals) > 0 {
			flagged = append(flagged, flaggedCommit{info: c, signals: signals})
		}
	}

	est.AICommits = int64(len(flagged))

	if est.TotalCommits > 0 {
		est.CommitPercent = float64(est.AICommits) / float64(est.TotalCommits) * 100
	}

	// Fetch stats for AI-flagged commits concurrently
	type commitDetail struct {
		index     int
		additions int64
		deletions int64
		err       error
	}

	details := make([]commitDetail, len(flagged))
	sem := make(chan struct{}, statsConcurrency)
	var wg sync.WaitGroup

	for i, fc := range flagged {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, hash string) {
			defer wg.Done()
			defer func() { <-sem }()
			add, del, err := cl.CommitStats(ctx, repo, hash)
			details[idx] = commitDetail{index: idx, additions: add, deletions: del, err: err}
		}(i, fc.info.Hash)
	}
	wg.Wait()

	var partialErrors []string
	for i, fc := range flagged {
		d := details[i]
		if d.err != nil {
			partialErrors = append(partialErrors, fmt.Sprintf("CommitStats %s: %v", fc.info.Hash, d.err))
			continue
		}

		est.AIAdditions += d.additions

		// Extract first line of commit message
		firstLine := fc.info.Message
		for j, ch := range fc.info.Message {
			if ch == '\n' {
				firstLine = fc.info.Message[:j]
				break
			}
		}

		est.Details = append(est.Details, model.AICommit{
			Hash:      fc.info.Hash,
			Author:    fc.info.Author,
			Message:   firstLine,
			Signals:   fc.signals,
			Additions: d.additions,
			Deletions: d.deletions,
		})
	}

	return est, partialErrors, nil
}
