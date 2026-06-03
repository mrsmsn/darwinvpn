package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/cli"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func TestVersion_PrintsVersionString(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := cli.NewRootCmd(vpn.NewFakeManager(nil))
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "darwinvpn") {
		t.Errorf("version output missing program name; got %q", out)
	}
	if !strings.Contains(out, cli.Version()) {
		t.Errorf("version output missing version %q; got %q", cli.Version(), out)
	}
}
