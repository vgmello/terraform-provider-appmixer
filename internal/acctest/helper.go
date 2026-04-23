package acctest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// SpawnMock starts the Bun mock server on a free port and sets
// APPMIXER_BASE_URL / APPMIXER_USERNAME / APPMIXER_PASSWORD for the
// lifetime of the test binary. Returns a cleanup func.
func SpawnMock(t *testing.T) func() {
	t.Helper()
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}

	root := repoRoot(t)
	cmd := exec.Command("bun", "run", "server.ts")
	cmd.Dir = filepath.Join(root, "mock-server")
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mock: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitReady(baseURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("mock not ready: %v", err)
	}

	_ = os.Setenv("APPMIXER_BASE_URL", baseURL)
	_ = os.Setenv("APPMIXER_USERNAME", "admin@test.com")
	_ = os.Setenv("APPMIXER_PASSWORD", "test123")

	return func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
}

// SpawnMockPackageLevel is like SpawnMock but for use in TestMain. It
// skips the t.Helper plumbing and panics on error.
func SpawnMockPackageLevel() func() {
	port, err := freePort()
	if err != nil {
		panic(err)
	}
	root, _ := os.Getwd()
	// walk up until we find mock-server/
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(root, "mock-server")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}
	cmd := exec.Command("bun", "run", "server.ts")
	cmd.Dir = filepath.Join(root, "mock-server")
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitReady(baseURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		panic(err)
	}
	_ = os.Setenv("APPMIXER_BASE_URL", baseURL)
	_ = os.Setenv("APPMIXER_USERNAME", "admin@test.com")
	_ = os.Setenv("APPMIXER_PASSWORD", "test123")
	_ = os.Setenv("TF_ACC", "1")
	return func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s", url)
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				return nil
			}
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0) // internal/acctest/helper.go
	return filepath.Join(filepath.Dir(file), "..", "..")
}
