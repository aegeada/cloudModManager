package harness

import (
	"bytes"
	"os/exec"
	"strings"
)

func (c *TestContext) Run(args ...string) *RunResult {
	return c.RunWithEnv(nil, "", args...)
}

func (c *TestContext) RunWithStdin(stdin string, args ...string) *RunResult {
	return c.RunWithEnv(nil, stdin, args...)
}

func (c *TestContext) RunWithEnv(env map[string]string, stdin string, args ...string) *RunResult {
	cmd := exec.Command(BinaryPath, args...)
	cmd.Dir = c.TempDir

	cmdEnv := append([]string{}, c.Env...)
	for k, v := range env {
		cmdEnv = append(cmdEnv, k+"="+v)
	}
	cmd.Env = cmdEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			c.T.Fatalf("failed to run command: %v", err)
		}
	}

	return &RunResult{
		T:        c.T,
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Ctx:      c,
	}
}
