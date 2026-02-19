# AI-Generated Narrative for Markdown Output

## Overview

Add an optional `--narrative` flag to the `markdown` command that pipes the JSON report through an AI CLI (claude, codex, or gemini) to generate a rich narrative analysis with groupings, derived metrics, outlier identification, and trend commentary.

## CLI Interface

```
codemium markdown --narrative report.json
codemium markdown --narrative --ai-cli gemini report.json
codemium markdown --narrative --ai-prompt "Focus on security repos" report.json
codemium markdown --narrative --ai-prompt-file custom-prompt.txt report.json
```

### New flags on `markdown` command

- `--narrative` — enable AI narrative generation (default: off)
- `--ai-cli <name>` — which CLI to use (`claude`, `codex`, `gemini`). Default: auto-detect first available.
- `--ai-prompt <text>` — additional instructions appended to the default prompt
- `--ai-prompt-file <path>` — read additional instructions from a file (mutually exclusive with `--ai-prompt`)

## Architecture

### Auto-detection (`internal/narrative` package)

`DetectCLI()` checks PATH for, in order: `claude`, `codex`, `gemini`. Returns the first found. If none found and `--narrative` is requested, return error listing supported CLIs.

### CLI execution

Each CLI is invoked in non-interactive mode with JSON piped via stdin:

| CLI | Command |
|-----|---------|
| claude | `claude -p "<prompt>"` (stdin piped) |
| codex | `codex exec "<prompt>"` (stdin piped) |
| gemini | `gemini -p "<prompt>"` (stdin piped) |

Stdout is captured as the narrative markdown output.

### Default prompt

Built-in prompt template that:
1. Describes the input format (codemium JSON — standard Report or TrendsReport)
2. For standard reports: group repos by project keys/naming patterns, compute derived metrics (comment ratios, etc.), identify outliers, write high-level overview + per-group breakdown with tables
3. For trends reports: analyze growth/decline patterns, identify fastest-growing repos/languages, comment on trajectory
4. Output pure markdown — no code fences, no preamble

The `--ai-prompt` / `--ai-prompt-file` content is appended to the default prompt as additional instructions.

### Report type handling

The narrative package receives raw JSON bytes. The default prompt tells the LLM to inspect the JSON structure to determine report type and adjust analysis accordingly.

### Output

When `--narrative` is used, the AI-generated narrative replaces the standard markdown output entirely. The LLM generates the complete document.
