package history

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GenerateDates produces a slice of target dates based on interval.
//
// For "monthly": since/until are "YYYY-MM" strings. Returns end-of-month
// dates (last day, 23:59:59 UTC) for each month from since to until inclusive.
//
// For "weekly": since/until are "YYYY-MM-DD" strings. Returns end-of-day
// dates (23:59:59 UTC) every 7 days from since to until inclusive.
func GenerateDates(since, until, interval string) []time.Time {
	switch interval {
	case "monthly":
		return generateMonthly(since, until)
	case "weekly":
		return generateWeekly(since, until)
	default:
		return nil
	}
}

func generateMonthly(since, until string) []time.Time {
	start, err := time.Parse("2006-01", since)
	if err != nil {
		return nil
	}
	end, err := time.Parse("2006-01", until)
	if err != nil {
		return nil
	}

	var dates []time.Time
	for cur := start; !cur.After(end); cur = cur.AddDate(0, 1, 0) {
		// End of month: go to first day of next month, subtract one day,
		// then set time to 23:59:59.
		nextMonth := cur.AddDate(0, 1, 0)
		lastDay := nextMonth.AddDate(0, 0, -1)
		eom := time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 0, time.UTC)
		dates = append(dates, eom)
	}
	return dates
}

func generateWeekly(since, until string) []time.Time {
	start, err := time.Parse("2006-01-02", since)
	if err != nil {
		return nil
	}
	end, err := time.Parse("2006-01-02", until)
	if err != nil {
		return nil
	}

	var dates []time.Time
	for cur := start; !cur.After(end); cur = cur.AddDate(0, 0, 7) {
		eod := time.Date(cur.Year(), cur.Month(), cur.Day(), 23, 59, 59, 0, time.UTC)
		dates = append(dates, eod)
	}
	return dates
}

// FormatPeriod formats a date according to the interval type.
//
// For "monthly": returns "2006-01" format.
// For "weekly": returns "2006-01-02" format.
func FormatPeriod(d time.Time, interval string) string {
	switch interval {
	case "monthly":
		return d.Format("2006-01")
	case "weekly":
		return d.Format("2006-01-02")
	default:
		return d.Format(time.RFC3339)
	}
}

// FindCommits walks the commit log from HEAD and, for each target date,
// finds the last commit whose Author.When is at or before that date.
// Dates with no prior commits are omitted from the result map.
func FindCommits(repo *git.Repository, dates []time.Time) (map[time.Time]plumbing.Hash, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, err
	}

	logIter, err := repo.Log(&git.LogOptions{
		From:  head.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, err
	}

	// Collect all commits sorted newest-first (default log order).
	var commits []*object.Commit
	err = logIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make(map[time.Time]plumbing.Hash)
	for _, targetDate := range dates {
		// Find the last commit at or before targetDate.
		// Commits are newest-first, so the first commit we find
		// with Author.When <= targetDate is the most recent one
		// at or before that date.
		for _, c := range commits {
			if !c.Author.When.After(targetDate) {
				result[targetDate] = c.Hash
				break
			}
		}
	}

	return result, nil
}
