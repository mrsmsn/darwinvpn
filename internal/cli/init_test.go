package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestInit_CreatesEmptyConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{{UUID: "u-1", Name: "alpha"}})
	out, err := runCLI(t, mgr, "--config", cfgPath, "init")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "created config:") {
		t.Errorf("missing created-config line: %q", out)
	}
	if !strings.Contains(out, "darwinvpn add") {
		t.Errorf("missing follow-up hint (non-TTY mode): %q", out)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != config.CurrentVersion {
		t.Errorf("version = %d, want %d", cfg.Version, config.CurrentVersion)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("Profiles = %d, want 0", len(cfg.Profiles))
	}
	info, _ := os.Stat(cfgPath)
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode = %o, want %o", got, want)
	}
}

func TestInit_RefusesIfExists(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := vpn.NewFakeManager(nil)
	_, err := runCLI(t, mgr, "--config", cfgPath, "init")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected already-exists error, got %v", err)
	}
}
