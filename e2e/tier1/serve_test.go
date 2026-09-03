package tier1

import (
	"cmm/e2e/harness"
	"cmm/e2e/mockserver"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestServe_ServesLockfileWithToken(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods]")

	cmd := exec.Command(harness.BinaryPath, "serve", "--port", "8081", "--token", "tok")
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(200 * time.Millisecond) // Give it time to start

	req, _ := http.NewRequest("GET", "http://127.0.0.1:8081/lock", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "[mods]" {
		t.Fatalf("unexpected lockfile content")
	}
}

func TestServe_UnauthorizedWithoutToken(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	ctx.WriteFile("cmm.lock", "[mods]")

	cmd := exec.Command(harness.BinaryPath, "serve", "--port", "8082", "--token", "tok")
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(200 * time.Millisecond)

	req, _ := http.NewRequest("GET", "http://127.0.0.1:8082/lock", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSyncRemote_PullFromServer(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	// We mock the remote server response for sync client test
	testSrv := mockserver.New() // could be just httptest.Server
	defer testSrv.Close()
	
	res := ctx.Run("sync", "--url", testSrv.URL(), "--token", "tok")
	res.AssertSuccess()
}

func TestServe_GracefulShutdown(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	cmd := exec.Command(harness.BinaryPath, "serve", "--port", "8083")
	cmd.Dir = ctx.TempDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start serve: %v", err)
	}
	
	time.Sleep(200 * time.Millisecond)
	
	// Send interrupt
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("failed to send interrupt: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		t.Fatal("serve did not shut down gracefully")
	case <-done:
		// success
	}
}

func TestSyncRemote_ServerUnreachable(t *testing.T) {
	srv := mockserver.New()
	defer srv.Close()
	ctx := harness.NewTestContext(t, srv.URL())

	res := ctx.Run("sync", "--url", "http://127.0.0.1:9999")
	res.AssertFailure()
}
