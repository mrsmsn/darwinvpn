// Package cli wires the cobra command tree for darwinvpn.
//
// The root command and each subcommand take a vpn.Manager so that tests can
// inject a fake. main() injects the real darwin-backed manager.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// NewRootCmd builds the root cobra command with all subcommands attached.
// The provided Manager is injected into every subcommand that needs it.
func NewRootCmd(mgr vpn.Manager) *cobra.Command {
	root := &cobra.Command{
		Use:           "darwinvpn",
		Short:         "Control macOS native IKEv2 VPN connections from the CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("config", "~/.config/darwinvpn/config.yaml", "path to config file")
	root.PersistentFlags().Bool("json", false, "machine-readable JSON output (for list/status)")
	root.PersistentFlags().BoolP("verbose", "v", false, "verbose logging")

	root.AddCommand(
		newListCmd(mgr),
		newStartCmd(mgr),
		newStopCmd(mgr),
		newStatusCmd(mgr),
		newAddCmd(mgr),
		newInitCmd(mgr),
		newVersionCmd(),
	)

	return root
}
