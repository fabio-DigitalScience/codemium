package license_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dsablic/codemium/internal/license"
)

func TestDetectMIT(t *testing.T) {
	dir := t.TempDir()
	mit := `MIT License

Copyright (c) 2024 Example

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
	os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(mit), 0644)

	result := license.Detect(dir)
	if result != "MIT" {
		t.Errorf("expected MIT, got %q", result)
	}
}

func TestDetectNoLicense(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	result := license.Detect(dir)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
