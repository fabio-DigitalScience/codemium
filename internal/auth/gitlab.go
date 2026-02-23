// internal/auth/gitlab.go
package auth

import (
	"os/exec"
	"strings"
)

// GlabCLIToken attempts to get a GitLab token from the glab CLI tool.
// Returns the token and true if successful, or empty string and false otherwise.
func GlabCLIToken() (string, bool) {
	out, err := exec.Command("glab", "config", "get", "token", "--host", "gitlab.com").Output()
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", false
	}
	return token, true
}
