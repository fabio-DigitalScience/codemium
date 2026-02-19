// internal/analyzer/clone_test.go
package analyzer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/dsablic/codemium/internal/analyzer"
)

func TestCloneAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping clone test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cloner := analyzer.NewCloner("", "")

	// Clone a small public repo
	dir, cleanup, err := cloner.Clone(ctx, "https://github.com/kelseyhightower/nocode.git")
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	// Verify directory exists and has files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read cloned dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("cloned directory is empty")
	}

	// Cleanup should remove the directory
	cleanup()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected directory to be removed after cleanup, but it still exists")
	}
}

func TestCheckout(t *testing.T) {
	// Create a temporary directory for a local git repo.
	tmpDir := t.TempDir()

	// Initialize a bare repo.
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("plain init: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	sig := &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Now(),
	}

	// First commit: create a.txt
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if _, err := wt.Add("a.txt"); err != nil {
		t.Fatalf("add a.txt: %v", err)
	}
	commit1, err := wt.Commit("add a.txt", &git.CommitOptions{Author: sig})
	if err != nil {
		t.Fatalf("commit 1: %v", err)
	}

	// Second commit: create b.txt
	if err := os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	if _, err := wt.Add("b.txt"); err != nil {
		t.Fatalf("add b.txt: %v", err)
	}
	if _, err := wt.Commit("add b.txt", &git.CommitOptions{Author: sig}); err != nil {
		t.Fatalf("commit 2: %v", err)
	}

	// Verify b.txt exists before checkout
	if _, err := os.Stat(filepath.Join(tmpDir, "b.txt")); err != nil {
		t.Fatalf("b.txt should exist before checkout: %v", err)
	}

	// Checkout back to commit1
	if err := analyzer.Checkout(repo, tmpDir, commit1); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// a.txt should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "a.txt")); err != nil {
		t.Errorf("a.txt should exist after checkout to commit1: %v", err)
	}

	// b.txt should NOT exist
	if _, err := os.Stat(filepath.Join(tmpDir, "b.txt")); !os.IsNotExist(err) {
		t.Errorf("b.txt should not exist after checkout to commit1, got err: %v", err)
	}
}
