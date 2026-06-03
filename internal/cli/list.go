package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newListCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered VPN profiles and their current status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
