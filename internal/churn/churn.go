package churn

import (
	"context"
	"sort"
	"sync"

	"github.com/dsablic/codemium/internal/model"
	"github.com/dsablic/codemium/internal/provider"
)

const (
	maxTopFiles      = 20
	statsConcurrency = 10
)

func Analyze(ctx context.Context, cl provider.ChurnLister, repo model.Repo, commitLimit int) (*model.ChurnStats, error) {
	commits, err := cl.ListCommits(ctx, repo, commitLimit)
	if err != nil {
		return nil, err
	}

	type commitFiles struct {
		files []provider.FileChange
		err   error
	}

	results := make([]commitFiles, len(commits))
	sem := make(chan struct{}, statsConcurrency)
	var wg sync.WaitGroup

	for i, c := range commits {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, hash string) {
			defer wg.Done()
			defer func() { <-sem }()
			files, err := cl.CommitFileStats(ctx, repo, hash)
			results[idx] = commitFiles{files: files, err: err}
		}(i, c.Hash)
	}
	wg.Wait()

	type fileAgg struct {
		changes   int64
		additions int64
		deletions int64
	}
	agg := map[string]*fileAgg{}

	for _, r := range results {
		if r.err != nil {
			continue
		}
		for _, f := range r.files {
			a, ok := agg[f.Path]
			if !ok {
				a = &fileAgg{}
				agg[f.Path] = a
			}
			a.changes++
			a.additions += f.Additions
			a.deletions += f.Deletions
		}
	}

	var topFiles []model.FileChurn
	for path, a := range agg {
		topFiles = append(topFiles, model.FileChurn{
			Path: path, Changes: a.changes, Additions: a.additions, Deletions: a.deletions,
		})
	}

	sort.Slice(topFiles, func(i, j int) bool {
		return topFiles[i].Changes > topFiles[j].Changes
	})

	if len(topFiles) > maxTopFiles {
		topFiles = topFiles[:maxTopFiles]
	}

	return &model.ChurnStats{
		TotalCommits: int64(len(commits)),
		TopFiles:     topFiles,
	}, nil
}

const maxHotspots = 10

func ComputeHotspots(files []model.FileChurn, complexity map[string]int64, limit int) []model.FileChurn {
	if limit <= 0 {
		limit = maxHotspots
	}

	var hotspots []model.FileChurn
	for _, f := range files {
		c, ok := complexity[f.Path]
		if !ok || c == 0 {
			continue
		}
		hotspots = append(hotspots, model.FileChurn{
			Path: f.Path, Changes: f.Changes, Additions: f.Additions, Deletions: f.Deletions,
			Complexity: c, Hotspot: float64(f.Changes) * float64(c),
		})
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].Hotspot > hotspots[j].Hotspot
	})

	if len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}
	return hotspots
}
