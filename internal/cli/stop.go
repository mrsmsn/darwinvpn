package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newStopCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a VPN connection (default profile if name omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
