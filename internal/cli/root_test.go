package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/cli"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newRoot(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	return buf
}

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

func TestSubcommands_ReturnNotImplemented(t *testing.T) {
	for _, name := range []string{"list", "start", "stop", "status", "add", "init"} {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{name})
			err := cmd.ExecuteContext(context.Background())
			if err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
			if !strings.Contains(err.Error(), "not yet implemented") {
				t.Errorf("%s: error = %q, want to contain 'not yet implemented'", name, err.Error())
			}
		})
	}
	_ = newRoot
}
