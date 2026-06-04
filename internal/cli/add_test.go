package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// Under `go test`, stdin/stdout are not terminals, so add without --use must
// fall back to the explanatory error rather than attempting the TUI.
func TestAdd_RequiresUseWhenNonInteractive(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{{UUID: "u-1", Name: "alpha"}})
	_, err := runCLI(t, mgr, "--config", cfgPath, "add")
	if err == nil || !strings.Contains(err.Error(), "TTY") {
		t.Errorf("expected non-TTY error mentioning --use, got %v", err)
	}
}

// --create (Mode B) requires interactive input, so it must surface the TTY
// error in non-interactive contexts instead of trying to open System
// Settings.
func TestAddCreate_RequiresTTY(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{{UUID: "u-1", Name: "alpha"}})
	_, err := runCLI(t, mgr, "--config", cfgPath, "add", "--create")
	if err == nil || !strings.Contains(err.Error(), "TTY") {
		t.Errorf("expected TTY error, got %v", err)
	}
}

func TestAdd_RegistersProfileFromSystemDisplayName(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "corp-vpn"},
	})
	out, err := runCLI(t, mgr, "--config", cfgPath, "add",
		"--use", "corp-vpn",
		"--name", "work-ghe",
		"--description", "Enterprise GitHub VPN",
		"--default")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "registered profile \"work-ghe\"") {
		t.Errorf("unexpected output: %q", out)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Default != "work-ghe" {
		t.Errorf("default = %q, want work-ghe", cfg.Default)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("Profiles = %d, want 1", len(cfg.Profiles))
	}
	p := cfg.Profiles[0]
	if p.Name != "work-ghe" || p.System.DisplayName != "corp-vpn" || p.System.UUID != "u-1" {
		t.Errorf("unexpected profile: %+v", p)
	}
	if p.Description != "Enterprise GitHub VPN" {
		t.Errorf("description not preserved: %q", p.Description)
	}
}

func TestAdd_NameDefaultsToDisplayName(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{{UUID: "u-1", Name: "Some VPN"}})
	if _, err := runCLI(t, mgr, "--config", cfgPath, "add", "--use", "u-1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.Profiles[0].Name != "Some-VPN" {
		t.Errorf("derived name = %q, want Some-VPN", cfg.Profiles[0].Name)
	}
}

func TestAdd_UnknownSystem(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{{UUID: "u-1", Name: "alpha"}})
	_, err := runCLI(t, mgr, "--config", cfgPath, "add", "--use", "nosuch")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestAdd_DuplicateRequiresForce(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha"},
		{UUID: "u-2", Name: "beta"},
	})
	if _, err := runCLI(t, mgr, "--config", cfgPath, "add", "--use", "alpha", "--name", "work"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	_, err := runCLI(t, mgr, "--config", cfgPath, "add", "--use", "beta", "--name", "work")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected duplicate error, got %v", err)
	}
	if _, err := runCLI(t, mgr, "--config", cfgPath, "add", "--use", "beta", "--name", "work", "--force"); err != nil {
		t.Fatalf("forced add: %v", err)
	}
	cfg, _ := config.Load(cfgPath)
	if cfg.Profiles[0].System.DisplayName != "beta" {
		t.Errorf("expected forced overwrite to point at beta, got %+v", cfg.Profiles[0].System)
	}
}
