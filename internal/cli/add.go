package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newAddCmd(mgr vpn.Manager) *cobra.Command {
	var (
		use         string
		name        string
		description string
		makeDefault bool
		force       bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register an existing system VPN as a darwinvpn profile (Mode A)",
		Long: "Register an existing system VPN as a darwinvpn profile.\n\n" +
			"Phase 1 only supports Mode A: pick a VPN that has already been\n" +
			"installed on the host (System Settings > Network or a previously\n" +
			"approved .mobileconfig) and record it in the YAML config. Mode B\n" +
			"(interactive mobileconfig generation) lands in Phase 2.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if use == "" {
				return errors.New("--use <name-or-uuid> is required (interactive mode arrives in Phase 2)")
			}
			services, err := mgr.List(cmd.Context())
			if err != nil {
				return err
			}
			svc, err := pickService(services, use)
			if err != nil {
				return err
			}
			if name == "" {
				name = defaultProfileName(svc.Name)
			}

			cfgPath, _ := cmd.Flags().GetString("config")
			if cfgPath == "" {
				cfgPath = config.DefaultPath()
			}
			cfg, err := loadOrInitConfig(cfgPath)
			if err != nil {
				return err
			}
			if err := upsertProfile(cfg, Profile{
				name:        name,
				description: description,
				system:      svc,
				force:       force,
			}); err != nil {
				return err
			}
			if makeDefault {
				cfg.Default = name
			}
			if err := config.Save(cfg, cfgPath); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(),
				"registered profile %q -> %s [%s]\nconfig: %s\n",
				name, svc.Name, svc.UUID, cfgPath)
			return err
		},
	}
	cmd.Flags().StringVar(&use, "use", "", "system VPN to register (display name or UUID)")
	cmd.Flags().StringVar(&name, "name", "", "profile name (default: derived from system display name)")
	cmd.Flags().StringVar(&description, "description", "", "free-form description for the profile")
	cmd.Flags().BoolVar(&makeDefault, "default", false, "make this profile the default")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing profile with the same name")
	return cmd
}

// Profile is a small internal projection used by upsertProfile so the CLI
// can stay loosely coupled to the YAML schema.
type Profile struct {
	name        string
	description string
	system      vpn.Service
	force       bool
}

func pickService(services []vpn.Service, needle string) (vpn.Service, error) {
	if len(services) == 0 {
		return vpn.Service{}, errNoProfiles
	}
	target := strings.ToLower(needle)
	for _, s := range services {
		if strings.ToLower(s.Name) == target || strings.ToLower(s.UUID) == target {
			return s, nil
		}
	}
	return vpn.Service{}, fmt.Errorf("system VPN %q not found (install it in System Settings first)", needle)
}

// defaultProfileName makes a config-friendly default for the YAML "name"
// field from the system display name. Whitespace is collapsed to hyphens
// and trimmed; everything else is kept as-is.
func defaultProfileName(display string) string {
	parts := strings.Fields(display)
	return strings.Join(parts, "-")
}

func loadOrInitConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if errors.Is(err, config.ErrNotFound) {
		return &config.Config{Version: config.CurrentVersion}, nil
	}
	if err != nil {
		return nil, err
	}
	if cfg.Version == 0 {
		cfg.Version = config.CurrentVersion
	}
	return cfg, nil
}

func upsertProfile(cfg *config.Config, p Profile) error {
	needle := strings.ToLower(p.name)
	for i := range cfg.Profiles {
		if strings.ToLower(cfg.Profiles[i].Name) == needle {
			if !p.force {
				return fmt.Errorf("profile %q already exists (use --force to overwrite)", p.name)
			}
			cfg.Profiles[i] = config.Profile{
				Name:        p.name,
				Description: p.description,
				System:      config.System{DisplayName: p.system.Name, UUID: p.system.UUID},
			}
			return nil
		}
	}
	cfg.Profiles = append(cfg.Profiles, config.Profile{
		Name:        p.name,
		Description: p.description,
		System:      config.System{DisplayName: p.system.Name, UUID: p.system.UUID},
	})
	return nil
}
