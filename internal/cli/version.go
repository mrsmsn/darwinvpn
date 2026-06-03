package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is overridden at build time via:
//
//	go build -ldflags "-X github.com/mrsmsn/darwinvpn/internal/cli.version=v0.0.1+sha"
var version = "dev"

// Version returns the version string baked into this build.
func Version() string { return version }

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print darwinvpn version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "darwinvpn %s\n", version)
			return err
		},
	}
}
