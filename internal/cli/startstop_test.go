package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestStart_SuccessTransitionsToConnected(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
	})
	out, err := runCLI(t, mgr, "start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(out, "starting alpha") {
		t.Errorf("unexpected output: %q", out)
	}
	got, _ := mgr.Status(context.Background(), "u-1")
	if got != vpn.StatusConnected {
		t.Errorf("post-start status = %s, want connected", got)
	}
}

func TestStart_AlreadyActiveReturnsSentinel(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
	})
	_, err := runCLI(t, mgr, "start")
	if !errors.Is(err, vpn.ErrAlreadyActive) {
		t.Errorf("expected errors.Is ErrAlreadyActive, got %v", err)
	}
}

func TestStop_SuccessTransitionsToDisconnected(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "stop")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(out, "stopping alpha") {
		t.Errorf("unexpected output: %q", out)
	}
	got, _ := mgr.Status(context.Background(), "u-1")
	if got != vpn.StatusDisconnected {
		t.Errorf("post-stop status = %s, want disconnected", got)
	}
}

func TestStop_NotActiveReturnsSentinel(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
	})
	_, err := runCLI(t, mgr, "stop")
	if !errors.Is(err, vpn.ErrNotActive) {
		t.Errorf("expected errors.Is ErrNotActive, got %v", err)
	}
}

func TestStart_NoProfiles(t *testing.T) {
	_, err := runCLI(t, vpn.NewFakeManager(nil), "start")
	if err == nil || !strings.Contains(err.Error(), "no IKEv2 VPN profile") {
		t.Errorf("expected no-profile error, got %v", err)
	}
}
