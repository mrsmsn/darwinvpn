package main

import (
	"fmt"
	"os"

	"github.com/mrsmsn/darwinvpn/internal/cli"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func main() {
	mgr, err := vpn.NewDarwinManager()
	if err != nil {
		// Phase 0: the bridge is a stub and returns ErrUnsupported.
		// Fall back to the fake so --help and version still work end-to-end.
		mgr = vpn.NewFakeManager(nil)
	}
	if err := cli.NewRootCmd(mgr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "darwinvpn:", err)
		os.Exit(1)
	}
}
