package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var BinaryPath string

func BuildBinary(tempDir string) error {
	binPath := filepath.Join(tempDir, "cmm")
	if envPath := os.Getenv("CMM_BIN_PATH"); envPath != "" {
		BinaryPath = envPath
		return nil
	}

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cmm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "../"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build cmm binary: %w", err)
	}
	BinaryPath = binPath
	return nil
}
