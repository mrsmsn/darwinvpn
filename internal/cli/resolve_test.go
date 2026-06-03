package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestResolve_ProfileByDisplayName(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
default: work
profiles:
  - name: work
    system:
      display_name: alpha
`)
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusDisconnected},
	})
	out, err := runCLI(t, mgr, "--config", cfg, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "alpha: connected") {
		t.Errorf("expected alpha resolved via profile; got %q", out)
	}
}

func TestResolve_ProfileByUUID(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
profiles:
  - name: only
    system:
      uuid: U-2
`)
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "--config", cfg, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "beta: connected") {
		t.Errorf("expected beta resolved by UUID; got %q", out)
	}
}

func TestResolve_NameNotInConfigFallsBackToDirect(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
profiles:
  - name: work
    system:
      display_name: alpha
`)
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusConnected},
	})
	// "beta" isn't a profile alias but is a real system service -> direct
	// match should still pick it up.
	out, err := runCLI(t, mgr, "--config", cfg, "status", "beta")
	if err != nil {
		t.Fatalf("status beta: %v", err)
	}
	if !strings.Contains(out, "beta: connected") {
		t.Errorf("expected fallback to direct match; got %q", out)
	}
}

func TestResolve_ProfilePointsToMissingService(t *testing.T) {
	cfg := writeConfig(t, `
version: 1
default: ghost
profiles:
  - name: ghost
    system:
      display_name: not-installed
`)
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
	})
	_, err := runCLI(t, mgr, "--config", cfg, "status")
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected missing-service error, got %v", err)
	}
}
