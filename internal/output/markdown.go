// internal/output/markdown.go
package output

import (
	"fmt"
	"io"
	"sort"

	"github.com/dsablic/codemium/internal/model"
)

// WriteMarkdown writes the report as GitHub-flavored markdown to w.
func WriteMarkdown(w io.Writer, report model.Report) error {
	fmt.Fprintf(w, "# Code Statistics Report\n\n")
	fmt.Fprintf(w, "**Provider:** %s\n", report.Provider)
	if report.Workspace != "" {
		fmt.Fprintf(w, "**Workspace:** %s\n", report.Workspace)
	}
	if report.Organization != "" {
		fmt.Fprintf(w, "**Organization:** %s\n", report.Organization)
	}
	fmt.Fprintf(w, "**Generated:** %s\n\n", report.GeneratedAt)

	// Summary totals
	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Metric | Value |\n")
	fmt.Fprintf(w, "|--------|-------|\n")
	fmt.Fprintf(w, "| Repositories | %d |\n", report.Totals.Repos)
	fmt.Fprintf(w, "| Files | %d |\n", report.Totals.Files)
	fmt.Fprintf(w, "| Lines | %d |\n", report.Totals.Lines)
	fmt.Fprintf(w, "| Code | %d |\n", report.Totals.Code)
	fmt.Fprintf(w, "| Comments | %d |\n", report.Totals.Comments)
	fmt.Fprintf(w, "| Blanks | %d |\n", report.Totals.Blanks)
	fmt.Fprintf(w, "| Complexity | %d |\n", report.Totals.Complexity)
	if report.Totals.FilteredFiles > 0 {
		fmt.Fprintf(w, "| Filtered Files | %d |\n", report.Totals.FilteredFiles)
	}
	fmt.Fprintln(w)

	// AI Code Estimation (only if present)
	if report.AIEstimate != nil {
		fmt.Fprintf(w, "## AI Code Estimation\n\n")
		fmt.Fprintf(w, "| Metric | Total | AI-Attributed | Percentage |\n")
		fmt.Fprintf(w, "|--------|------:|:-------------:|-----------:|\n")
		fmt.Fprintf(w, "| Commits | %d | %d | %.1f%% |\n",
			report.AIEstimate.TotalCommits, report.AIEstimate.AICommits, report.AIEstimate.CommitPercent)
		if report.AIEstimate.AIAdditions > 0 {
			fmt.Fprintf(w, "| Line additions | — | %d | — |\n", report.AIEstimate.AIAdditions)
		}
		fmt.Fprintln(w)
	}

	// By language
	fmt.Fprintf(w, "## Languages\n\n")
	fmt.Fprintf(w, "| Language | Files | Code | Comments | Blanks | Complexity |\n")
	fmt.Fprintf(w, "|----------|------:|-----:|---------:|-------:|-----------:|\n")
	for _, lang := range report.ByLanguage {
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d |\n",
			lang.Name, lang.Files, lang.Code, lang.Comments, lang.Blanks, lang.Complexity)
	}
	fmt.Fprintln(w)

	// Per repository
	hasAI := report.AIEstimate != nil
	fmt.Fprintf(w, "## Repositories\n\n")
	if hasAI {
		fmt.Fprintf(w, "| Repository | Project | License | Files | Code | Comments | Complexity | AI Commits %% | AI Additions |\n")
		fmt.Fprintf(w, "|------------|---------|---------|------:|-----:|---------:|-----------:|-------------:|-------------:|\n")
	} else {
		fmt.Fprintf(w, "| Repository | Project | License | Files | Code | Comments | Complexity |\n")
		fmt.Fprintf(w, "|------------|---------|---------|------:|-----:|---------:|-----------:|\n")
	}
	for _, repo := range report.Repositories {
		lic := repo.License
		if lic == "" {
			lic = "\u2014"
		}
		if hasAI {
			aiPct := "\u2014"
			aiAdd := "\u2014"
			if repo.AIEstimate != nil {
				aiPct = fmt.Sprintf("%.1f%%", repo.AIEstimate.CommitPercent)
				aiAdd = fmt.Sprintf("%d", repo.AIEstimate.AIAdditions)
			}
			fmt.Fprintf(w, "| [%s](%s) | %s | %s | %d | %d | %d | %d | %s | %s |\n",
				repo.Repository, repo.URL, repo.Project, lic, repo.Totals.Files, repo.Totals.Code,
				repo.Totals.Comments, repo.Totals.Complexity, aiPct, aiAdd)
		} else {
			fmt.Fprintf(w, "| [%s](%s) | %s | %s | %d | %d | %d | %d |\n",
				repo.Repository, repo.URL, repo.Project, lic, repo.Totals.Files, repo.Totals.Code,
				repo.Totals.Comments, repo.Totals.Complexity)
		}
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

// WriteTrendsMarkdown writes the trends report as GitHub-flavored markdown to w.
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

	langSet := map[string]bool{}
	for _, snap := range report.Snapshots {
		for _, lang := range snap.ByLanguage {
			langSet[lang.Name] = true
		}
	}
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

	if len(report.Errors) > 0 {
		fmt.Fprintf(w, "## Errors\n\n")
		for _, e := range report.Errors {
			fmt.Fprintf(w, "- **%s**: %s\n", e.Repository, e.Error)
		}
		fmt.Fprintln(w)
	}

	return nil
}
