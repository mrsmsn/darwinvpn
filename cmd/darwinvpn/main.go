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
		// Non-darwin platforms (or builds with CGO disabled) get a fake so
		// --help and version still work; functional subcommands will report
		// ErrUnsupported on the actual call.
		mgr = vpn.NewFakeManager(nil)
	}
	if err := cli.NewRootCmd(mgr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "darwinvpn:", err)
		os.Exit(1)
	}
}
