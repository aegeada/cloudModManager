package loader

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ProcessChecker can be overridden in tests.
var ProcessChecker func() bool = isRunningDefault

func isRunningDefault() bool {
	// If CMM_MOCK_SERVER_RUNNING env is set to "1" or "true", simulate running process
	if val := os.Getenv("CMM_MOCK_SERVER_RUNNING"); val == "1" || strings.EqualFold(val, "true") {
		return true
	}

	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist").Output()
		if err == nil {
			lower := strings.ToLower(string(out))
			if strings.Contains(lower, "javaw.exe") || strings.Contains(lower, "java.exe") || strings.Contains(lower, "minecraft") {
				return true
			}
		}
		return false
	}

	// Linux / macOS
	out, err := exec.Command("pgrep", "-f", "java").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}

	out2, err2 := exec.Command("pgrep", "-f", "minecraft").Output()
	if err2 == nil && len(strings.TrimSpace(string(out2))) > 0 {
		return true
	}

	return false
}

// IsServerOrMinecraftRunning checks whether a Minecraft server or client java process is running.
func IsServerOrMinecraftRunning() bool {
	if ProcessChecker != nil {
		return ProcessChecker()
	}
	return isRunningDefault()
}
