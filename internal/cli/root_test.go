package cli_test

import (
	"bytes"
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

// add and init are both functional in Phase 2. Their own tests live in
// add_test.go and init_test.go.
