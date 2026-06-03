package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/cli"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func runCLI(t *testing.T, mgr vpn.Manager, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd := cli.NewRootCmd(mgr)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestList_NoProfiles(t *testing.T) {
	out, err := runCLI(t, vpn.NewFakeManager(nil), "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "no IKEv2 VPN profile registered") {
		t.Errorf("expected empty-state message; got %q", out)
	}
}

func TestList_TableContainsServices(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusDisconnected},
		{UUID: "u-2", Name: "beta", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{"NAME", "STATUS", "UUID", "alpha", "beta", "disconnected", "connected", "u-1", "u-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestList_JSON(t *testing.T) {
	mgr := vpn.NewFakeManager([]vpn.Service{
		{UUID: "u-1", Name: "alpha", Status: vpn.StatusConnected},
	})
	out, err := runCLI(t, mgr, "--json", "list")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var parsed []struct {
		Name   string `json:"name"`
		UUID   string `json:"uuid"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%s", err, out)
	}
	if len(parsed) != 1 || parsed[0].Name != "alpha" || parsed[0].Status != "connected" {
		t.Errorf("unexpected JSON payload: %+v", parsed)
	}
}
