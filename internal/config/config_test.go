package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/config"
)

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoad_MinimalConfig(t *testing.T) {
	path := writeFile(t, "config.yaml", `
version: 1
default: work-ghe
profiles:
  - name: work-ghe
    system:
      display_name: corp-vpn
`)
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Default != "work-ghe" {
		t.Errorf("default = %q, want work-ghe", c.Default)
	}
	if len(c.Profiles) != 1 || c.Profiles[0].System.DisplayName != "corp-vpn" {
		t.Errorf("unexpected profiles: %+v", c.Profiles)
	}
}

func TestLoad_FullSchema(t *testing.T) {
	path := writeFile(t, "config.yaml", `
version: 1
default: work-ghe
profiles:
  - name: work-ghe
    description: Enterprise GitHub VPN
    system:
      display_name: corp-vpn
      uuid: 00000000-0000-0000-0000-000000000001
    ikev2:
      server: vpn.example.com
      remote_id: vpn.example.com
      local_id: user@example.com
      auth: eap
      on_demand:
        enabled: true
        match_domains: [ghe.example.com]
    secret:
      provider: keychain
      ref: darwinvpn/work-ghe
`)
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := c.Profiles[0]
	if p.IKEv2 == nil || p.IKEv2.Server != "vpn.example.com" || p.IKEv2.OnDemand == nil || !p.IKEv2.OnDemand.Enabled {
		t.Errorf("ikev2 mis-parsed: %+v", p.IKEv2)
	}
	if p.Secret == nil || p.Secret.Provider != "keychain" || p.Secret.Ref != "darwinvpn/work-ghe" {
		t.Errorf("secret mis-parsed: %+v", p.Secret)
	}
	if p.System.UUID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("uuid mis-parsed: %q", p.System.UUID)
	}
}

func TestLoad_MissingFileReturnsErrNotFound(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestLoad_RejectsOlderSchema(t *testing.T) {
	path := writeFile(t, "config.yaml", "version: -1\nprofiles: []\n")
	_, err := config.Load(path)
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Errorf("expected version error, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	src := &config.Config{
		Version: 1,
		Default: "work-ghe",
		Profiles: []config.Profile{
			{
				Name:   "work-ghe",
				System: config.System{DisplayName: "corp-vpn"},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "subdir", "config.yaml")
	if err := config.Save(src, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Default != src.Default || len(got.Profiles) != 1 ||
		got.Profiles[0].Name != "work-ghe" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode = %o, want %o", got, want)
	}
}

func TestProfileByName_DefaultFallback(t *testing.T) {
	c := &config.Config{
		Default: "work-ghe",
		Profiles: []config.Profile{
			{Name: "personal"},
			{Name: "work-ghe"},
		},
	}
	p, err := c.ProfileByName("")
	if err != nil {
		t.Fatalf("ProfileByName: %v", err)
	}
	if p.Name != "work-ghe" {
		t.Errorf("got %q, want work-ghe", p.Name)
	}
}

func TestProfileByName_SingleProfileNoDefault(t *testing.T) {
	c := &config.Config{
		Profiles: []config.Profile{{Name: "only"}},
	}
	p, err := c.ProfileByName("")
	if err != nil {
		t.Fatalf("ProfileByName: %v", err)
	}
	if p.Name != "only" {
		t.Errorf("got %q, want only", p.Name)
	}
}

func TestProfileByName_AmbiguousWithoutDefault(t *testing.T) {
	c := &config.Config{
		Profiles: []config.Profile{{Name: "a"}, {Name: "b"}},
	}
	_, err := c.ProfileByName("")
	if err == nil || !strings.Contains(err.Error(), "multiple profiles") {
		t.Errorf("got %v, want multiple-profiles error", err)
	}
}

func TestProfileByName_Unknown(t *testing.T) {
	c := &config.Config{Profiles: []config.Profile{{Name: "a"}}}
	_, err := c.ProfileByName("nosuch")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v, want not-found error", err)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home: %v", err)
	}
	cases := map[string]string{
		"~":           home,
		"~/foo":       filepath.Join(home, "foo"),
		"/tmp/x":      "/tmp/x",
		"./relative":  "./relative",
	}
	for in, want := range cases {
		got, err := config.ExpandPath(in)
		if err != nil {
			t.Errorf("ExpandPath(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ExpandPath(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := config.ExpandPath(""); err == nil {
		t.Error("ExpandPath(\"\") expected error")
	}
}
