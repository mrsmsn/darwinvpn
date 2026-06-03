package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestStatus_DefaultSingleProfile(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "alpha: connected") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestStatus_AmbiguousWithoutName(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusDisconnected},
	})
	_, err := runCLI(t, mgr, "status")
	if err == nil || !strings.Contains(err.Error(), "multiple profiles") {
		t.Errorf("expected ambiguity error, got %v", err)
	}
}

func TestStatus_NoProfiles(t *testing.T) {
	_, err := runCLI(t, vpn.NewFakeManager(nil), "status")
	if err == nil || !strings.Contains(err.Error(), "no VPN profile") {
		t.Errorf("expected no-profile error, got %v", err)
	}
}

func TestStatus_NameLookup(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "status", "beta")
	if err != nil {
		t.Fatalf("status beta: %v", err)
	}
	if !strings.Contains(out, "beta: connected") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestStatus_UnknownName(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
	})
	_, err := runCLI(t, mgr, "status", "nosuch")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestStatus_JSON(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "--json", "status")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var parsed struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput=%s", err, out)
	}
	if parsed.Name != "alpha" || parsed.Status != "connected" {
		t.Errorf("unexpected JSON: %+v", parsed)
	}
}
