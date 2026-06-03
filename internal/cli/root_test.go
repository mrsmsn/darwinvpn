package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/cli"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestRoot_HelpListsAllSubcommands(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	want := []string{"list", "start", "stop", "status", "add", "init", "version"}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("--help output missing %q\n--- output ---\n%s", w, out)
		}
	}
}

func TestRoot_PersistentFlagsAreDefined(t *testing.T) {
	cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
	for _, name := range []string{"config", "json", "verbose"} {
		if f := cmd.PersistentFlags().Lookup(name); f == nil {
			t.Errorf("persistent flag --%s not defined", name)
		}
	}
}

// init is still a stub in Phase 1; verify it continues to surface
// errNotImplemented until Phase 2 lands. add is functional via flags now;
// its own tests live in add_test.go.
func TestInit_StillStub(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("init: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("init: error = %q, want 'not yet implemented'", err.Error())
	}
}
