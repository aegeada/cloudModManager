package harness

import (
	"os"
	"path/filepath"
	"testing"
)

type TestContext struct {
	T       *testing.T
	TempDir string
	Env     []string
	BaseURL string
}

func NewTestContext(t *testing.T, mockServerURL string) *TestContext {
	tempDir := t.TempDir()

	env := []string{
		"MODRINTH_API_URL=" + mockServerURL + "/v2",
		"FABRIC_META_URL=" + mockServerURL + "/fabric-meta",
		"HOME=" + tempDir,
		"TERM=dumb",
		"CMM_NO_SELF_INSTALL=1",
	}

	return &TestContext{
		T:       t,
		TempDir: tempDir,
		Env:     env,
		BaseURL: mockServerURL,
	}
}

func (c *TestContext) WriteFile(name, content string) {
	path := filepath.Join(c.TempDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.T.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		c.T.Fatalf("failed to write file %s: %v", name, err)
	}
}

func (c *TestContext) WriteBytes(name string, data []byte) {
	path := filepath.Join(c.TempDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.T.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		c.T.Fatalf("failed to write file %s: %v", name, err)
	}
}

func (c *TestContext) ReadFile(name string) string {
	path := filepath.Join(c.TempDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		c.T.Fatalf("failed to read file %s: %v", name, err)
	}
	return string(data)
}

func (c *TestContext) SetEnv(key, value string) {
	c.Env = append(c.Env, key+"="+value)
}

