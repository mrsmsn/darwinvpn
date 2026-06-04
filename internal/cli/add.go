package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// addOpts is the input surface of runAdd. The CLI command populates it from
// cobra flags; init reuses runAdd directly to chain interactive add after
// config creation.
type addOpts struct {
	cfgPath         string
	use             string
	create          bool
	name            string
	description     string
	makeDefault     bool
	force           bool
	defaultExplicit bool
}

func newAddCmd(mgr vpn.Manager) *cobra.Command {
	opts := addOpts{}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a system VPN as a darwinvpn profile",
		Long: "Register a system VPN (one already installed via System Settings or\n" +
			"a previously approved .mobileconfig) as a darwinvpn profile.\n\n" +
			"Run without --use to pick the target interactively via a TUI.\n" +
			"Pass --use <name-or-uuid> to skip the prompts (useful from scripts\n" +
			"and non-interactive environments).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.cfgPath, _ = cmd.Flags().GetString("config")
			if opts.cfgPath == "" {
				opts.cfgPath = config.DefaultPath()
			}
			opts.defaultExplicit = cmd.Flags().Changed("default")
			return runAdd(cmd.Context(), cmd.OutOrStdout(), mgr, &opts)
		},
	}
	cmd.Flags().StringVar(&opts.use, "use", "", "system VPN to register (display name or UUID); omit for interactive mode")
	cmd.Flags().BoolVar(&opts.create, "create", false, "create a new IKEv2 VPN by generating a .mobileconfig (TTY required)")
	cmd.Flags().StringVar(&opts.name, "name", "", "profile name (default: derived from system display name)")
	cmd.Flags().StringVar(&opts.description, "description", "", "free-form description for the profile")
	cmd.Flags().BoolVar(&opts.makeDefault, "default", false, "make this profile the default")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing profile with the same name")
	return cmd
}

func runAdd(ctx context.Context, out io.Writer, mgr vpn.Manager, opts *addOpts) error {
	// Mode B short-circuit: --create skips the existing-VPN listing entirely.
	if opts.create {
		return runAddCreate(ctx, out, mgr, opts)
	}

	services, err := mgr.List(ctx)
	if err != nil {
		return err
	}

	interactive := opts.use == ""
	if interactive && !isTTY() {
		return errors.New("--use is required when stdin/stdout is not a TTY")
	}

	// When the user runs `darwinvpn add` interactively and the system has
	// at least one VPN already installed, ask whether they want Mode A
	// (pick one) or Mode B (create a new one). With zero installed VPNs
	// the only useful answer is Mode B, so jump straight there.
	if interactive {
		if len(services) == 0 {
			opts.create = true
			return runAddCreate(ctx, out, mgr, opts)
		}
		create, err := promptMode()
		if err != nil {
			return err
		}
		if create {
			opts.create = true
			return runAddCreate(ctx, out, mgr, opts)
		}
	} else if len(services) == 0 {
		return errNoProfiles
	}

	var svc vpn.Service
	if interactive {
		if svc, err = selectServiceInteractive(services); err != nil {
			return err
		}
	} else {
		if svc, err = pickService(services, opts.use); err != nil {
			return err
		}
	}

	if opts.name == "" {
		opts.name = defaultProfileName(svc.Name)
	}
	if interactive {
		if opts.name, err = promptName(opts.name); err != nil {
			return err
		}
		if opts.description == "" {
			if opts.description, err = promptDescription(); err != nil {
				return err
			}
		}
		if !opts.defaultExplicit {
			if opts.makeDefault, err = promptMakeDefault(); err != nil {
				return err
			}
		}
	}

	cfg, err := loadOrInitConfig(opts.cfgPath)
	if err != nil {
		return err
	}
	if err := upsertProfile(cfg, Profile{
		name:        opts.name,
		description: opts.description,
		system:      svc,
		force:       opts.force,
	}); err != nil {
		return err
	}
	if opts.makeDefault {
		cfg.Default = opts.name
	}
	if err := config.Save(cfg, opts.cfgPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out,
		"registered profile %q -> %s [%s]\nconfig: %s\n",
		opts.name, svc.Name, svc.UUID, opts.cfgPath)
	return err
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

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func selectServiceInteractive(services []vpn.Service) (vpn.Service, error) {
	opts := make([]huh.Option[string], 0, len(services))
	byUUID := make(map[string]vpn.Service, len(services))
	for _, s := range services {
		label := fmt.Sprintf("%s  (%s)", s.Name, s.Status)
		opts = append(opts, huh.NewOption(label, s.UUID))
		byUUID[s.UUID] = s
	}
	chosen := services[0].UUID
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a system VPN to register").
				Options(opts...).
				Value(&chosen),
		),
	)
	if err := form.Run(); err != nil {
		return vpn.Service{}, err
	}
	return byUUID[chosen], nil
}

func promptName(initial string) (string, error) {
	val := initial
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Profile name").
				Description("Alias used by darwinvpn start/stop/status").
				Value(&val).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("name cannot be empty")
					}
					return nil
				}),
		),
	)
	if err := f.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

func promptDescription() (string, error) {
	var val string
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description").
				Description("Optional free-form note (press Enter to skip)").
				Value(&val),
		),
	)
	if err := f.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(val), nil
}

func promptMakeDefault() (bool, error) {
	var v bool
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Make this the default profile?").
				Affirmative("Yes").
				Negative("No").
				Value(&v),
		),
	)
	if err := f.Run(); err != nil {
		return false, err
	}
	return v, nil
}
