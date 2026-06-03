package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newListCmd(mgr vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered VPN profiles and their current status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			services, err := mgr.List(cmd.Context())
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return writeServicesJSON(cmd.OutOrStdout(), services)
			}
			return writeServicesTable(cmd.OutOrStdout(), services)
		},
	}
}

type serviceJSON struct {
	Name   string `json:"name"`
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

func writeServicesJSON(w io.Writer, services []vpn.Service) error {
	out := make([]serviceJSON, len(services))
	for i, s := range services {
		out[i] = serviceJSON{Name: s.Name, UUID: s.UUID, Status: s.Status.String()}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeServicesTable(w io.Writer, services []vpn.Service) error {
	if len(services) == 0 {
		_, err := fmt.Fprintln(w, "(no VPN profile registered)")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tSTATUS\tUUID"); err != nil {
		return err
	}
	for _, s := range services {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Status, s.UUID); err != nil {
			return err
		}
	}
	return tw.Flush()
}
