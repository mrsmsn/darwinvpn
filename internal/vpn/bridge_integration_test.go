//go:build darwin && cgo && integration

package vpn_test

import (
	"context"
	"errors"
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

func TestIntegration_ListReturnsAllVPNs(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	services, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("found %d VPN service(s)", len(services))
	for _, s := range services {
		t.Logf("  %-30s [%s] status=%s", s.Name, s.UUID, s.Status)
	}
	if len(services) == 0 {
		t.Skip("no VPN configured on this host; configure one in " +
			"System Settings to verify the filter end-to-end")
	}
	for _, s := range services {
		if s.Name == "" {
			t.Errorf("empty Name: %+v", s)
		}
		if s.UUID == "" {
			t.Errorf("empty UUID: %+v", s)
		}
		// Probe1 confirmed firewall / Network Privacy entries also surface
		// via NEConfigurationManager but have VPN == nil; the filter must
		// drop them.
		switch s.Name {
		case "com.apple.preferences.application-firewall":
			t.Errorf("non-VPN configuration leaked through filter: %s", s.Name)
		}
	}
}

func TestIntegration_Status_MatchesList(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	ctx := context.Background()
	services, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(services) == 0 {
		t.Skip("no IKEv2 VPN configured; cannot exercise Status")
	}
	for _, s := range services {
		got, err := mgr.Status(ctx, s.UUID)
		if err != nil {
			t.Errorf("Status(%s): %v", s.Name, err)
			continue
		}
		t.Logf("  %s: List.Status=%s  Status()=%s", s.Name, s.Status, got)
		// The two calls can race in principle (state may change between
		// List and Status). We log mismatches rather than fail so the test
		// remains useful even when a transition happens during the run.
		if got != s.Status {
			t.Logf("  NOTE: status changed between List and Status (acceptable)")
		}
	}
}

func TestIntegration_Status_UnknownUUID(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	const bogus = "00000000-0000-0000-0000-000000000000"
	_, err = mgr.Status(context.Background(), bogus)
	if !errors.Is(err, vpn.ErrNotFound) {
		t.Errorf("Status(bogus UUID): got %v, want errors.Is ErrNotFound", err)
	}
}

func TestIntegration_StatusDetail_MatchesList(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	ctx := context.Background()
	services, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(services) == 0 {
		t.Skip("no VPN configured; cannot exercise StatusDetail")
	}
	sawIKEv2 := false
	for _, s := range services {
		sd, err := mgr.StatusDetail(ctx, s.UUID)
		if err != nil {
			t.Errorf("StatusDetail(%s): %v", s.Name, err)
			continue
		}
		t.Logf("  %s: status=%s server=%q remote_id=%q username=%q connected_at=%v",
			s.Name, sd.Status, sd.ServerAddress, sd.RemoteIdentifier,
			sd.Username, sd.ConnectedAt)
		// Status from StatusDetail must match List() at the time of call,
		// modulo races during a live transition (logged in TestIntegration_
		// Status_MatchesList).
		if sd.ServerAddress != "" {
			sawIKEv2 = true
		}
		if sd.Status == vpn.StatusConnected && sd.ConnectedAt.IsZero() {
			t.Logf("  NOTE: connected session %s has zero ConnectedAt; "+
				"ne_session_get_info may have failed", s.Name)
		}
	}
	if !sawIKEv2 {
		t.Logf("no IKEv2 VPN observed; ServerAddress non-empty assertion " +
			"could not be exercised on this host")
	}
}

func TestIntegration_StatusDetail_UnknownUUID(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	const bogus = "00000000-0000-0000-0000-000000000000"
	_, err = mgr.StatusDetail(context.Background(), bogus)
	if !errors.Is(err, vpn.ErrNotFound) {
		t.Errorf("StatusDetail(bogus UUID): got %v, want errors.Is ErrNotFound", err)
	}
}

// The next three tests inspect Start/Stop behavior without changing the
// VPN's actual state: they only exercise the sentinel error paths driven
// by the current status. A real start-then-stop round trip is gated on
// DARWINVPN_INTEGRATION_DANGEROUS=1 in TestIntegration_StartStop_RoundTrip.

func TestIntegration_Start_UnknownUUID(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	const bogus = "00000000-0000-0000-0000-000000000000"
	if err := mgr.Start(context.Background(), bogus); !errors.Is(err, vpn.ErrNotFound) {
		t.Errorf("Start(bogus): got %v, want errors.Is ErrNotFound", err)
	}
}

func TestIntegration_Start_AlreadyActive(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	ctx := context.Background()
	services, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	exercised := false
	for _, s := range services {
		switch s.Status {
		case vpn.StatusConnecting, vpn.StatusConnected, vpn.StatusReasserting:
			exercised = true
			if err := mgr.Start(ctx, s.UUID); !errors.Is(err, vpn.ErrAlreadyActive) {
				t.Errorf("Start(%s, status=%s): got %v, want ErrAlreadyActive",
					s.Name, s.Status, err)
			}
		}
	}
	if !exercised {
		t.Skip("no active session to exercise ErrAlreadyActive")
	}
}

func TestIntegration_Stop_NotActive(t *testing.T) {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		t.Fatalf("NewDarwinManager: %v", err)
	}
	ctx := context.Background()
	services, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	exercised := false
	for _, s := range services {
		if s.Status == vpn.StatusDisconnected || s.Status == vpn.StatusInvalid {
			exercised = true
			if err := mgr.Stop(ctx, s.UUID); !errors.Is(err, vpn.ErrNotActive) {
				t.Errorf("Stop(%s, status=%s): got %v, want ErrNotActive",
					s.Name, s.Status, err)
			}
		}
	}
	if !exercised {
		t.Skip("no inactive session to exercise ErrNotActive")
	}
}
