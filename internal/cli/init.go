package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newInitCmd(_ vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a darwinvpn config file and add the first profile (Phase 2)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
