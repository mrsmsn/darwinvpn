package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newStartCmd(mgr vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "start [name]",
		Short: "Start a VPN connection (default profile if name omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			svc, err := resolveService(cmd.Context(), mgr, name)
			if err != nil {
				return err
			}
			if err := mgr.Start(cmd.Context(), svc.UUID); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "starting %s\n", svc.Name)
			return err
		},
	}
}

// Aliases match docs/pj.md §7's optional connect/disconnect verbs.
var _ = vpn.Manager(nil)
