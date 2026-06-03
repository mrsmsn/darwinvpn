package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newAddCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Interactively create a new VPN profile (Phase 2)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
