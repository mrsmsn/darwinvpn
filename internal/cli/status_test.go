package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestStatus_DefaultSingleProfile(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{{
		UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected,
		ServerAddress: "vpn.example.com", RemoteIdentifier: "vpn.example.com",
		Username: "alice", ConnectedAt: time.Now().Add(-12*time.Minute - 34*time.Second),
	}})
	out, err := runCLI(t, mgr, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, line := range []string{
		"Name:        alpha",
		"Status:      connected",
		"Server:      vpn.example.com",
		"Remote ID:   vpn.example.com",
		"Username:    alice",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("output missing %q\nfull output:\n%s", line, out)
		}
	}
	if !strings.Contains(out, "Connected:   ") {
		t.Errorf("expected Connected line, got:\n%s", out)
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
		{UUID: "u-2", Name: "beta", Status: vpn.StatusConnected,
			ServerAddress: "vpn.example.com"},
	})
	out, err := runCLI(t, mgr, "status", "beta")
	if err != nil {
		t.Fatalf("status beta: %v", err)
	}
	if !strings.Contains(out, "Name:        beta") {
		t.Errorf("missing Name line: %s", out)
	}
	if !strings.Contains(out, "Status:      connected") {
		t.Errorf("missing Status line: %s", out)
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

func TestStatus_DisconnectedHidesConnectedAndDynamic(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{{
		UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected,
		ServerAddress: "vpn.example.com", RemoteIdentifier: "vpn.example.com",
		Username: "alice",
	}})
	out, err := runCLI(t, mgr, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "Server:      vpn.example.com") {
		t.Errorf("static fields should be shown when disconnected, got:\n%s", out)
	}
	if strings.Contains(out, "Connected:") {
		t.Errorf("Connected line must not appear when disconnected, got:\n%s", out)
	}
}

func TestStatus_OmitsEmptyFields(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{{
		UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected,
		ServerAddress: "vpn.example.com",
	}})
	out, err := runCLI(t, mgr, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(out, "Remote ID:") {
		t.Errorf("empty Remote ID line should be omitted, got:\n%s", out)
	}
	if strings.Contains(out, "Username:") {
		t.Errorf("empty Username line should be omitted, got:\n%s", out)
	}
}

func TestStatus_JSON_AllFields(t *testing.T) {
	connectedAt := time.Date(2026, 6, 4, 8, 12, 34, 0, time.UTC)
	mgr := vpn.NewFakeManager([]vpn.Service{{
		UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected,
		ServerAddress: "vpn.example.com", RemoteIdentifier: "vpn.example.com",
		Username: "alice", ConnectedAt: connectedAt,
	}})
	out, err := runCLI(t, mgr, "--json", "status")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var parsed struct {
		Name        string  `json:"name"`
		UUID        string  `json:"uuid"`
		Status      string  `json:"status"`
		Server      string  `json:"server"`
		RemoteID    string  `json:"remote_id"`
		Username    string  `json:"username"`
		ConnectedAt *string `json:"connected_at"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput=%s", err, out)
	}
	if parsed.Name != "alpha" || parsed.Status != "connected" ||
		parsed.Server != "vpn.example.com" || parsed.RemoteID != "vpn.example.com" ||
		parsed.Username != "alice" {
		t.Errorf("unexpected JSON: %+v", parsed)
	}
	if parsed.ConnectedAt == nil || *parsed.ConnectedAt != "2026-06-04T08:12:34Z" {
		t.Errorf("connected_at: got %v, want 2026-06-04T08:12:34Z", parsed.ConnectedAt)
	}
}

func TestStatus_JSON_NullConnectedAtWhenDisconnected(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{{
		UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected,
		ServerAddress: "vpn.example.com",
	}})
	out, err := runCLI(t, mgr, "--json", "status")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var parsed struct {
		ConnectedAt *string `json:"connected_at"`
		Server      string  `json:"server"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput=%s", err, out)
	}
	if parsed.ConnectedAt != nil {
		t.Errorf("connected_at should be null when disconnected, got %v", *parsed.ConnectedAt)
	}
	if parsed.Server != "vpn.example.com" {
		t.Errorf("server should be reported when disconnected, got %q", parsed.Server)
	}
}
