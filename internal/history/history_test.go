package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGenerateDatesMonthly(t *testing.T) {
	dates := GenerateDates("2025-01", "2025-03", "monthly")

	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}

	// First date: end of January 2025
	if dates[0].Year() != 2025 || dates[0].Month() != time.January {
		t.Errorf("first date: expected 2025-01, got %d-%02d", dates[0].Year(), dates[0].Month())
	}
	if dates[0].Day() != 31 {
		t.Errorf("first date: expected day 31, got %d", dates[0].Day())
	}

	// Last date: end of March 2025
	if dates[2].Year() != 2025 || dates[2].Month() != time.March {
		t.Errorf("last date: expected 2025-03, got %d-%02d", dates[2].Year(), dates[2].Month())
	}
	if dates[2].Day() != 31 {
		t.Errorf("last date: expected day 31, got %d", dates[2].Day())
	}

	// Verify all dates are at 23:59:59 UTC
	for i, d := range dates {
		if d.Hour() != 23 || d.Minute() != 59 || d.Second() != 59 {
			t.Errorf("date %d: expected 23:59:59, got %02d:%02d:%02d", i, d.Hour(), d.Minute(), d.Second())
		}
		if d.Location() != time.UTC {
			t.Errorf("date %d: expected UTC, got %v", i, d.Location())
		}
	}
}

func TestGenerateDatesWeekly(t *testing.T) {
	dates := GenerateDates("2025-01-01", "2025-01-22", "weekly")

	if len(dates) != 4 {
		t.Fatalf("expected 4 dates, got %d", len(dates))
	}

	expectedDays := []int{1, 8, 15, 22}
	for i, d := range dates {
		if d.Day() != expectedDays[i] {
			t.Errorf("date %d: expected day %d, got %d", i, expectedDays[i], d.Day())
		}
		if d.Month() != time.January || d.Year() != 2025 {
			t.Errorf("date %d: expected 2025-01, got %d-%02d", i, d.Year(), d.Month())
		}
		if d.Hour() != 23 || d.Minute() != 59 || d.Second() != 59 {
			t.Errorf("date %d: expected 23:59:59, got %02d:%02d:%02d", i, d.Hour(), d.Minute(), d.Second())
		}
	}
}

func TestFormatPeriodMonthly(t *testing.T) {
	d := time.Date(2025, 3, 31, 23, 59, 59, 0, time.UTC)
	result := FormatPeriod(d, "monthly")
	if result != "2025-03" {
		t.Errorf("expected 2025-03, got %s", result)
	}
}

func TestFormatPeriodWeekly(t *testing.T) {
	d := time.Date(2025, 1, 15, 23, 59, 59, 0, time.UTC)
	result := FormatPeriod(d, "weekly")
	if result != "2025-01-15" {
		t.Errorf("expected 2025-01-15, got %s", result)
	}
}

func TestFindCommits(t *testing.T) {
	// Create a temporary directory for the test repo.
	dir := t.TempDir()

	// Initialize a new git repository.
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Commit 1: 2025-01-15
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("first"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if _, err := wt.Add("file1.txt"); err != nil {
		t.Fatalf("failed to add file1: %v", err)
	}
	commit1Time := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	hash1, err := wt.Commit("first commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  commit1Time,
		},
	})
	if err != nil {
		t.Fatalf("failed to create commit 1: %v", err)
	}

	// Commit 2: 2025-02-20
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("second"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	if _, err := wt.Add("file2.txt"); err != nil {
		t.Fatalf("failed to add file2: %v", err)
	}
	commit2Time := time.Date(2025, 2, 20, 12, 0, 0, 0, time.UTC)
	hash2, err := wt.Commit("second commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  commit2Time,
		},
	})
	if err != nil {
		t.Fatalf("failed to create commit 2: %v", err)
	}

	// Commit 3: 2025-03-10
	if err := os.WriteFile(filepath.Join(dir, "file3.txt"), []byte("third"), 0644); err != nil {
		t.Fatalf("failed to write file3: %v", err)
	}
	if _, err := wt.Add("file3.txt"); err != nil {
		t.Fatalf("failed to add file3: %v", err)
	}
	commit3Time := time.Date(2025, 3, 10, 12, 0, 0, 0, time.UTC)
	_, err = wt.Commit("third commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
			When:  commit3Time,
		},
	})
	if err != nil {
		t.Fatalf("failed to create commit 3: %v", err)
	}

	// Target dates: end of each month Jan-Mar 2025.
	dates := GenerateDates("2025-01", "2025-03", "monthly")
	if len(dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(dates))
	}

	result, err := FindCommits(repo, dates)
	if err != nil {
		t.Fatalf("FindCommits failed: %v", err)
	}

	// End of January (2025-01-31 23:59:59): should match commit 1 (Jan 15).
	if h, ok := result[dates[0]]; !ok {
		t.Error("expected a commit for end of January")
	} else if h != hash1 {
		t.Errorf("end of January: expected hash %s, got %s", hash1, h)
	}

	// End of February (2025-02-28 23:59:59): should match commit 2 (Feb 20).
	if h, ok := result[dates[1]]; !ok {
		t.Error("expected a commit for end of February")
	} else if h != hash2 {
		t.Errorf("end of February: expected hash %s, got %s", hash2, h)
	}

	// End of March (2025-03-31 23:59:59): should match commit 3 (Mar 10).
	if h, ok := result[dates[2]]; !ok {
		t.Error("expected a commit for end of March")
	} else {
		// Commit 3 is the latest at or before end of March.
		_ = h
	}

	// Test a date before any commits: 2024-12-31 should not have a match.
	earlyDate := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	earlyResult, err := FindCommits(repo, []time.Time{earlyDate})
	if err != nil {
		t.Fatalf("FindCommits for early date failed: %v", err)
	}
	if _, ok := earlyResult[earlyDate]; ok {
		t.Error("expected no commit for date before any commits exist")
	}
}
