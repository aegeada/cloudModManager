package selfinstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AutoInstall checks if the running binary is outside PATH and installs it to ~/.local/bin/cmm.
func AutoInstall() {
	if os.Getenv("CMM_NO_SELF_INSTALL") == "1" {
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return
	}

	execDir := filepath.Dir(execPath)
	execName := filepath.Base(execPath)

	targetBinName := "cmm"
	if runtime.GOOS == "windows" {
		targetBinName = "cmm.exe"
	}

	// Determine user's local bin directory
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return
	}

	targetDir := filepath.Join(homeDir, ".local", "bin")
	targetPath := filepath.Join(targetDir, targetBinName)

	// If current executable is already the target binary in targetDir, nothing to do
	if strings.EqualFold(execPath, targetPath) {
		return
	}

	// Check if the current executable is already in system PATH under targetBinName
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)
	alreadyInPath := false
	for _, p := range paths {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(execDir)) && strings.EqualFold(execName, targetBinName) {
			alreadyInPath = true
			break
		}
	}
	if alreadyInPath {
		return
	}

	// Perform copy to targetDir
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return
	}

	srcFile, err := os.Open(execPath)
	if err != nil {
		return
	}
	defer srcFile.Close()

	// Write to temporary file first then atomic rename
	tmpFile := targetPath + ".tmp"
	dstFile, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		_ = os.Remove(tmpFile)
		return
	}
	dstFile.Close()

	if err := os.Rename(tmpFile, targetPath); err != nil {
		_ = os.Remove(targetPath)
		if err := os.Rename(tmpFile, targetPath); err != nil {
			return
		}
	}
	_ = os.Chmod(targetPath, 0755)

	// Check if targetDir is in PATH
	inPath := false
	for _, p := range paths {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(targetDir)) {
			inPath = true
			break
		}
	}

	fmt.Printf("\n✨ [Auto-Setup] Successfully installed 'cmm' to %s\n", targetPath)

	if !inPath {
		// Attempt to add to ~/.bashrc or ~/.profile
		addedToShell := false
		shellFiles := []string{
			filepath.Join(homeDir, ".bashrc"),
			filepath.Join(homeDir, ".zshrc"),
			filepath.Join(homeDir, ".profile"),
		}

		exportLine := fmt.Sprintf("\n# Added by Cloud Mod Manager (cmm)\nexport PATH=\"%s:$PATH\"\n", targetDir)

		for _, sf := range shellFiles {
			if data, err := os.ReadFile(sf); err == nil {
				if !strings.Contains(string(data), ".local/bin") {
					f, err := os.OpenFile(sf, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						_, _ = f.WriteString(exportLine)
						f.Close()
						addedToShell = true
					}
				} else {
					addedToShell = true
				}
				break
			}
		}

		if addedToShell {
			fmt.Printf("💡 PATH has been updated in your shell profile. Run 'source ~/.bashrc' or restart your terminal to use 'cmm' from any directory!\n\n")
		} else {
			fmt.Printf("💡 To use 'cmm' from anywhere, add this to your ~/.bashrc:\n   export PATH=\"%s:$PATH\"\n\n", targetDir)
		}
	} else {
		fmt.Printf("💡 You can now run 'cmm' directly from any folder in your terminal!\n\n")
	}
}

// ManualInstall explicitly installs the executable to ~/.local/bin/cmm.
func ManualInstall() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return fmt.Errorf("failed to determine home directory")
	}

	targetDir := filepath.Join(homeDir, ".local", "bin")
	targetBinName := "cmm"
	if runtime.GOOS == "windows" {
		targetBinName = "cmm.exe"
	}
	targetPath := filepath.Join(targetDir, targetBinName)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	srcFile, err := os.Open(execPath)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %w", targetPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	_ = os.Chmod(targetPath, 0755)

	return nil
}
