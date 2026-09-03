package harness

import (
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type RunResult struct {
	T        *testing.T
	ExitCode int
	Stdout   string
	Stderr   string
	Ctx      *TestContext
}

func (r *RunResult) AssertSuccess() {
	r.T.Helper()
	if r.ExitCode != 0 {
		r.T.Fatalf("expected success, got exit code %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

func (r *RunResult) AssertFailure(expectedExitCode ...int) {
	r.T.Helper()
	if r.ExitCode == 0 {
		r.T.Fatalf("expected failure, got success\nstdout: %s\nstderr: %s", r.Stdout, r.Stderr)
	}
	if len(expectedExitCode) > 0 {
		if r.ExitCode != expectedExitCode[0] {
			r.T.Fatalf("expected exit code %d, got %d", expectedExitCode[0], r.ExitCode)
		}
	}
}

func (r *RunResult) AssertStdoutContains(substr ...string) {
	r.T.Helper()
	for _, s := range substr {
		if !strings.Contains(r.Stdout, s) {
			r.T.Fatalf("expected stdout to contain %q\nstdout: %s", s, r.Stdout)
		}
	}
}

func (r *RunResult) AssertStdoutMatches(pattern string) {
	r.T.Helper()
	matched, err := regexp.MatchString(pattern, r.Stdout)
	if err != nil {
		r.T.Fatalf("invalid regexp pattern: %v", err)
	}
	if !matched {
		r.T.Fatalf("expected stdout to match pattern %q\nstdout: %s", pattern, r.Stdout)
	}
}

func (r *RunResult) AssertStderrContains(substr ...string) {
	r.T.Helper()
	for _, s := range substr {
		if !strings.Contains(r.Stderr, s) {
			r.T.Fatalf("expected stderr to contain %q\nstderr: %s", s, r.Stderr)
		}
	}
}

func (r *RunResult) AssertFileSHA512(path, expectedHash string) {
	r.T.Helper()
	fullPath := filepath.Join(r.Ctx.TempDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		r.T.Fatalf("failed to read file %s: %v", path, err)
	}
	hash := sha512.Sum512(data)
	actualHash := hex.EncodeToString(hash[:])
	if actualHash != expectedHash {
		r.T.Fatalf("expected SHA512 %s for file %s, got %s", expectedHash, path, actualHash)
	}
}
