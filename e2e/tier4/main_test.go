package tier4

import (
	"cmm/e2e/harness"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "cmm_e2e_tier4_*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	os.Chdir("..")
	if err := harness.BuildBinary(tempDir); err != nil {
		os.Exit(1)
	}
	os.Chdir("tier4")
	os.Exit(m.Run())
}
