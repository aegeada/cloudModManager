package harness
import "os"
func init() {
    if p := os.Getenv("CMM_BIN_PATH"); p != "" {
        BinaryPath = p
    }
}
