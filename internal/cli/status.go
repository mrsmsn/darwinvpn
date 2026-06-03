package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newStatusCmd(mgr vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show the current status of a VPN profile",
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
			st, err := mgr.Status(cmd.Context(), svc.UUID)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(serviceJSON{
					Name:   svc.Name,
					UUID:   svc.UUID,
					Status: st.String(),
				})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", svc.Name, st)
			return err
		},
	}
}
