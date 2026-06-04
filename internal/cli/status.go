package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

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
			svc, err := resolveService(cmd, mgr, name)
			if err != nil {
				return err
			}
			sd, err := mgr.StatusDetail(cmd.Context(), svc.UUID)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return writeStatusJSON(cmd.OutOrStdout(), svc, sd)
			}
			return writeStatusText(cmd.OutOrStdout(), svc, sd, time.Now())
		},
	}
}

type statusDetailJSON struct {
	Name        string     `json:"name"`
	UUID        string     `json:"uuid"`
	Status      string     `json:"status"`
	Server      string     `json:"server"`
	RemoteID    string     `json:"remote_id"`
	Username    string     `json:"username"`
	IPv4        string     `json:"ipv4"`
	IPv6        string     `json:"ipv6"`
	ConnectedAt *time.Time `json:"connected_at"`
}

func writeStatusJSON(w io.Writer, svc vpn.Service, sd vpn.StatusDetail) error {
	out := statusDetailJSON{
		Name:     svc.Name,
		UUID:     svc.UUID,
		Status:   sd.Status.String(),
		Server:   sd.ServerAddress,
		RemoteID: sd.RemoteIdentifier,
		Username: sd.Username,
		IPv4:     sd.IPv4Address,
		IPv6:     sd.IPv6Address,
	}
	if sd.Status == vpn.StatusConnected && !sd.ConnectedAt.IsZero() {
		t := sd.ConnectedAt.UTC()
		out.ConnectedAt = &t
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// writeStatusText writes the multi-line text status. `now` is injected so the
// elapsed-time output is deterministic in tests.
func writeStatusText(w io.Writer, svc vpn.Service, sd vpn.StatusDetail, now time.Time) error {
	rows := [][2]string{
		{"Name:", svc.Name},
		{"Status:", sd.Status.String()},
		{"Server:", sd.ServerAddress},
		{"Remote ID:", sd.RemoteIdentifier},
		{"Username:", sd.Username},
		{"IPv4:", sd.IPv4Address},
		{"IPv6:", sd.IPv6Address},
	}
	if sd.Status == vpn.StatusConnected && !sd.ConnectedAt.IsZero() {
		rows = append(rows, [2]string{"Connected:", formatElapsed(now.Sub(sd.ConnectedAt))})
	}
	for _, r := range rows {
		if r[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "%-12s %s\n", r[0], r[1]); err != nil {
			return err
		}
	}
	return nil
}

// formatElapsed renders a duration as "Xs", "Xm Ys", or "Xh Ym". Negative
// durations are clamped to zero so a brief clock skew between the system's
// connectedDate and now doesn't produce nonsense like "-1s".
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		s := int((d % time.Minute) / time.Second)
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%dh %dm", h, m)
}
