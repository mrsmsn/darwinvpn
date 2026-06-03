package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newStartCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "start [name]",
		Short: "Start a VPN connection (default profile if name omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
