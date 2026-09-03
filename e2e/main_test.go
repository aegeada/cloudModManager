package e2e

import (
	"cmm/e2e/harness"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "cmm_e2e_*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	if err := harness.BuildBinary(tempDir); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}
