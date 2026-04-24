//go:build e2e

// Package e2e drives the real terraform CLI against a built provider binary
// and the in-process mock server. It exercises every resource and data
// source, then walks plan -> apply -> apply-with-update -> destroy.
//
// Run with: go test -tags e2e -v ./e2e/...
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ellosoft/terraform-provider-appmixer/internal/mockserver"
)

// stackDir returns the absolute path to examples/stack based on this test file.
func stackDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "examples", "stack")
}

// buildProvider compiles the provider into dir/terraform-provider-appmixer.
func buildProvider(t *testing.T, dir string) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..")
	out := filepath.Join(dir, "terraform-provider-appmixer")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build provider: %v", err)
	}
}

// writeDevTFRC writes a Terraform CLI config that shortcuts the provider
// registry lookup and uses the local binary in binDir.
func writeDevTFRC(t *testing.T, path, binDir string) {
	t.Helper()
	content := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "ellosoft/appmixer" = %q
  }
  direct {}
}
`, binDir)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write tfrc: %v", err)
	}
}

// terraformEnv returns env bindings for the terraform subprocess.
func terraformEnv(baseURL, tfrcPath string) []string {
	env := os.Environ()
	env = append(env,
		"TF_CLI_CONFIG_FILE="+tfrcPath,
		"APPMIXER_BASE_URL="+baseURL,
		"APPMIXER_USERNAME=admin@test.com",
		"APPMIXER_PASSWORD=test123",
		// Silences the dev-override banner on every invocation.
		"TF_IN_AUTOMATION=1",
	)
	return env
}

// runTerraform runs the terraform CLI in workdir with the given args. On
// failure it dumps stdout+stderr to the test log.
func runTerraform(t *testing.T, workdir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("terraform", args...)
	cmd.Dir = workdir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, prefixWriter{t: t, prefix: "tf stdout"})
	cmd.Stderr = io.MultiWriter(&stderr, prefixWriter{t: t, prefix: "tf stderr"})
	if err := cmd.Run(); err != nil {
		t.Fatalf("terraform %s failed: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String()
}

type prefixWriter struct {
	t      *testing.T
	prefix string
}

func (w prefixWriter) Write(b []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line != "" {
			w.t.Logf("[%s] %s", w.prefix, line)
		}
	}
	return len(b), nil
}

// copyDir recursively copies src -> dst (files only, no symlinks).
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			copyDir(t, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
			continue
		}
		in, err := os.Open(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("open %s: %v", e.Name(), err)
		}
		out, err := os.Create(filepath.Join(dst, e.Name()))
		if err != nil {
			in.Close()
			t.Fatalf("create %s: %v", e.Name(), err)
		}
		if _, err := io.Copy(out, in); err != nil {
			t.Fatalf("copy %s: %v", e.Name(), err)
		}
		in.Close()
		out.Close()
	}
}

// readOutputs runs `terraform output -json` and returns the parsed map.
func readOutputs(t *testing.T, workdir string, env []string) map[string]struct {
	Value any `json:"value"`
} {
	t.Helper()
	out := runTerraform(t, workdir, env, "output", "-json")
	var parsed map[string]struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse terraform output: %v\n%s", err, out)
	}
	return parsed
}

func TestE2E_FullStack(t *testing.T) {
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("terraform CLI not on PATH")
	}

	// Start mock server in-process.
	baseURL, stopMock := mockserver.Start()
	t.Cleanup(stopMock)
	t.Logf("mock server: %s", baseURL)

	// Tempdir with provider binary + stack copy + tfrc.
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	buildProvider(t, binDir)

	tfrc := filepath.Join(tmp, "dev.tfrc")
	writeDevTFRC(t, tfrc, binDir)

	workdir := filepath.Join(tmp, "stack")
	copyDir(t, stackDir(t), workdir)

	env := terraformEnv(baseURL, tfrc)

	// ---- plan from empty state ----
	t.Log("step 1: initial plan (expect create for every resource)")
	planOut := runTerraform(t, workdir, env, "plan", "-no-color")
	for _, kind := range []string{
		"appmixer_system_config",
		"appmixer_service_config",
		"appmixer_modifiers",
		"appmixer_acl",
		"appmixer_account",
		"appmixer_user",
		"appmixer_flow",
		"appmixer_quota",
	} {
		if !strings.Contains(planOut, kind) {
			t.Errorf("plan output missing %s", kind)
		}
	}

	// ---- apply ----
	t.Log("step 2: apply")
	runTerraform(t, workdir, env, "apply", "-auto-approve", "-no-color")

	outs := readOutputs(t, workdir, env)
	for _, k := range []string{"user_id", "flow_id", "account_id", "quota_id", "readback_user", "readback_flow"} {
		v, ok := outs[k]
		if !ok || v.Value == nil || v.Value == "" {
			t.Errorf("missing/empty output %q: %+v", k, v)
		}
	}
	if got := outs["quota_is_custom"].Value; got != true {
		t.Errorf("quota_is_custom: want true, got %v", got)
	}
	if got := outs["readback_user"].Value; got != outs["user_id"].Value {
		t.Errorf("readback_user (%v) != user_id (%v)", got, outs["user_id"].Value)
	}
	if got := outs["readback_flow"].Value; got != outs["flow_id"].Value {
		t.Errorf("readback_flow (%v) != flow_id (%v)", got, outs["flow_id"].Value)
	}
	initialUserID := outs["user_id"].Value

	// ---- plan-no-op: a second plan on unchanged config should show no changes ----
	t.Log("step 3: idempotency plan (expect no changes)")
	planOut2 := runTerraform(t, workdir, env, "plan", "-no-color", "-detailed-exitcode")
	// -detailed-exitcode returns 2 when changes are planned. runTerraform exits
	// the test on nonzero, so reaching here means exit 0 (no changes) already.
	if strings.Contains(planOut2, "will be updated") || strings.Contains(planOut2, "will be replaced") {
		t.Errorf("expected no changes on second plan, got:\n%s", planOut2)
	}

	// ---- password rotation (in-place update via POST /user/reset-password) ----
	t.Log("step 4: rotate user password (in-place update)")
	runTerraform(t, workdir, env, "apply", "-auto-approve", "-no-color",
		"-var", "user_password=second-pass")

	outs2 := readOutputs(t, workdir, env)
	if got := outs2["user_id"].Value; got != initialUserID {
		t.Errorf("user_id changed across password update: was %v, now %v (should be in-place)", initialUserID, got)
	}

	// ---- quota source update (in-place update via PUT /quota/:name) ----
	t.Log("step 5: update quota source")
	newSrc := "module.exports = { rules: [{ limit: 999, window: 60000, resource: 'requests' }] };"
	runTerraform(t, workdir, env, "apply", "-auto-approve", "-no-color",
		"-var", "user_password=second-pass",
		"-var", "quota_source="+newSrc)

	// ---- destroy ----
	t.Log("step 6: destroy")
	runTerraform(t, workdir, env, "destroy", "-auto-approve", "-no-color",
		"-var", "user_password=second-pass",
		"-var", "quota_source="+newSrc)
}
