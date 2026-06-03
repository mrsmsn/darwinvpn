package cli

import (
	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newInitCmd(mgr vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a darwinvpn config file (Phase 2: interactive setup)",
		Long: "Phase 1 hosts only the scaffolding for init. The Phase 2 implementation\n" +
			"will create ~/.config/darwinvpn/config.yaml and then chain into\n" +
			"interactive `add`.",
		RunE: func(_ *cobra.Command, _ []string) error {
			_ = mgr
			return errNotImplemented
		},
	}
}
