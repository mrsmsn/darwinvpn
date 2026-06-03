//go:build darwin && cgo && integration

package vpn_test

import (
	"context"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// Run with: go test -tags integration -count=1 ./internal/vpn/...
//
// These tests hit the real NetworkExtension private APIs and require a live
// macOS host. They are read-only: List and Status are exercised, but Start
// and Stop are not, so running this on a developer machine cannot disrupt
// existing VPN sessions.

func TestIntegration_NewDarwinManager(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewDarwinManager returned nil manager")
	}
}

func TestIntegration_ListReturnsOnlyIKEv2(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	services, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("found %d IKEv2 service(s)", len(services))
	for _, s := range services {
		t.Logf("  %-30s [%s] status=%s", s.Name, s.UUID, s.Status)
	}
	if len(services) == 0 {
		t.Skip("no IKEv2 VPN configured on this host; configure one in " +
			"System Settings to verify the filter end-to-end")
	}
	for _, s := range services {
		if s.Name == "" {
			t.Errorf("empty Name: %+v", s)
		}
		if s.UUID == "" {
			t.Errorf("empty UUID: %+v", s)
		}
		// The probe confirmed firewall / network-privacy / Tunnel Provider
		// entries also show up via NEConfigurationManager; if the filter is
		// correct, none of those should appear here.
		switch s.Name {
		case "com.apple.preferences.application-firewall", "Tailscale":
			t.Errorf("non-IKEv2 configuration leaked through filter: %s", s.Name)
		}
	}
}
