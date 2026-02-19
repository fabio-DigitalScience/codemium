// internal/narrative/narrative.go
package narrative

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// supportedCLIs is the ordered list of AI CLI tools we can invoke.
var supportedCLIs = []string{"claude", "codex", "gemini"}

// SupportedCLIs returns the list of supported AI CLI tool names.
func SupportedCLIs() []string {
	out := make([]string, len(supportedCLIs))
	copy(out, supportedCLIs)
	return out
}

// LookupFunc resolves a command name to its path. Compatible with exec.LookPath.
type LookupFunc func(name string) (string, error)

// DetectCLI finds the first supported AI CLI available on the system PATH.
func DetectCLI() (string, error) {
	return DetectCLIWith(exec.LookPath)
}

// DetectCLIWith finds the first supported AI CLI using the provided lookup function.
// It iterates the supported CLIs in order and returns the first one the lookup finds.
// Returns an error if none of the supported CLIs are found.
func DetectCLIWith(lookup LookupFunc) (string, error) {
	for _, cli := range supportedCLIs {
		if _, err := lookup(cli); err == nil {
			return cli, nil
		}
	}
	return "", fmt.Errorf("no supported AI CLI found; install one of: %s", strings.Join(supportedCLIs, ", "))
}

// BuildArgs returns the command name and argument slice for a non-interactive
// invocation of the given CLI with the provided prompt.
func BuildArgs(cli, prompt string) (string, []string) {
	switch cli {
	case "codex":
		return "codex", []string{"exec", prompt}
	case "gemini":
		return "gemini", []string{"-p", prompt}
	default: // "claude" and fallback
		return "claude", []string{"-p", prompt}
	}
}

// DefaultPrompt returns a built-in prompt that instructs an AI CLI to generate
// a narrative markdown report from a JSON code-statistics report on stdin.
// If extra is non-empty it is appended as additional instructions.
func DefaultPrompt(extra string) string {
	var b strings.Builder

	b.WriteString(`You are a technical writer. You will receive a JSON report on stdin containing code statistics for a set of repositories.

The JSON has one of two shapes:
1. **Standard report** — top-level key "repositories" with an array of per-repo stats, plus "totals" and "by_language".
2. **Trends report** — top-level key "snapshots" with an array of period snapshots, each containing "repositories", "totals", and "by_language".

Analyze the data and produce a polished Markdown document. Output ONLY pure Markdown — no code fences wrapping the entire output, no preamble, no commentary outside the document.

Format all numbers with comma separators (e.g. 1,234,567). Compute derived metrics where useful, such as comment-to-code ratio (comments / code, as a percentage).

### For a standard report

1. **Title**: "# Code Statistics Report"
2. **Summary** (## Summary): 2–3 paragraphs giving a high-level overview. Group repositories by project or naming patterns. Identify outliers (largest/smallest repos, highest complexity). Note language distribution across the organization.
3. **Summary by Product Area** (## Summary by Product Area): A table grouping repos by their "project" field (or inferred grouping). Columns: Area, Repos, Files, Code, Comments, Comment %, Complexity.
4. **Top 10 Repositories** (## Top 10 Repositories): Table of the 10 largest repos by code lines. Columns: Rank, Repository, Code, Comments, Comment %, Files, Complexity.
5. **Per-Area Sections** (## <Area Name>): For each product area, a section with a table listing every repo in that area. Columns: Repository, Code, Comments, Comment %, Files, Top Language.

### For a trends report

1. **Title**: "# Code Statistics Trends"
2. **Overview** (## Overview): Summarize overall growth or decline in code, files, and complexity across all periods.
3. **Growth Analysis** (## Growth Analysis): Identify the fastest-growing repositories and languages. Note any inflection points or trend reversals.
4. **Summary Table** (## Period Summary): Table with one row per period. Columns: Period, Repos, Files, Code, Comments, Complexity, Delta Code, Delta %.
5. **Language Trends** (## Language Trends): Table showing how the top languages changed over time.
6. **Notable Changes** (## Notable Changes): Bullet list of the most significant changes between periods.
`)

	if extra != "" {
		b.WriteString("\n### Additional Instructions\n\n")
		b.WriteString(extra)
		b.WriteString("\n")
	}

	return b.String()
}

// Generate runs the specified AI CLI, pipes jsonData to its stdin, and returns
// the generated narrative markdown from stdout.
func Generate(ctx context.Context, cli string, jsonData []byte, prompt string) (string, error) {
	name, args := BuildArgs(cli, prompt)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(jsonData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return "", fmt.Errorf("%s failed: %w: %s", cli, err, strings.TrimSpace(errMsg))
		}
		return "", fmt.Errorf("%s failed: %w", cli, err)
	}

	return stdout.String(), nil
}
