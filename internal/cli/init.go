package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newInitCmd(mgr vpn.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a darwinvpn config file and register the first profile",
		Long: "Create ~/.config/darwinvpn/config.yaml (or --config path) and chain\n" +
			"into interactive `add` so the first profile is registered in the\n" +
			"same flow. If stdin/stdout is not a TTY, only the config file is\n" +
			"created and you are pointed at `darwinvpn add --use ...` for the\n" +
			"profile step.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			if cfgPath == "" {
				cfgPath = config.DefaultPath()
			}
			expanded, err := config.ExpandPath(cfgPath)
			if err != nil {
				return err
			}
			if _, err := os.Stat(expanded); err == nil {
				return fmt.Errorf("config already exists at %s; remove it first to re-init", expanded)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			empty := &config.Config{Version: config.CurrentVersion}
			if err := config.Save(empty, cfgPath); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "created config: %s\n\n", expanded); err != nil {
				return err
			}

			if !isTTY() {
				_, err := fmt.Fprintf(cmd.OutOrStdout(),
					"Next: run `darwinvpn add --use <system-name-or-uuid>` to register a profile.\n")
				return err
			}
			return runAdd(cmd.Context(), cmd.OutOrStdout(), mgr, &addOpts{cfgPath: cfgPath})
		},
	}
}
