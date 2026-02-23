// internal/auth/gitlab_test.go
package auth

import (
	"testing"
)

func TestGlabCLITokenNotInstalled(t *testing.T) {
	// If glab is not installed, GlabCLIToken should return false.
	// This test verifies it doesn't panic or hang.
	_, ok := GlabCLIToken()
	// We can't assert the exact result since glab may or may not be installed,
	// but we verify the function returns without error.
	_ = ok
}
