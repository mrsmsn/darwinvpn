package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newStatusCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show the current status of a VPN profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
