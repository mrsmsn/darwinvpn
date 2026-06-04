package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/provision"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// installTimeout caps how long runAddCreate waits for the user to approve
// the mobileconfig in System Settings. 5 minutes is the same ballpark the
// macOS UI uses before its own session gives up.
var installTimeout = 5 * time.Minute

// installAndDetect is a package-level indirection so tests can swap in a
// fake without spawning the real install flow.
var installAndDetect = provision.InstallAndDetect

// runAddCreate implements Mode B: gather IKEv2 + EAP parameters
// interactively, generate a .mobileconfig, hand it to System Settings via
// `open`, wait for the new NEConfiguration to surface, and then register a
// YAML profile against it.
func runAddCreate(ctx context.Context, out io.Writer, mgr vpn.Manager, opts *addOpts) error {
	if !isTTY() {
		return errors.New("--create requires a TTY for interactive IKEv2 input")
	}

	in, err := promptIKEv2Input()
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(out, "Opening installer; approve the profile in System Settings..."); err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	listFn := func(ctx context.Context) ([]vpn.Service, error) {
		return mgr.List(ctx)
	}
	svc, err := installAndDetect(ctxTimeout, in, listFn, 2*time.Second)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Detected new VPN: %s [%s]\n\n", svc.Name, svc.UUID); err != nil {
		return err
	}

	if opts.name == "" {
		opts.name = defaultProfileName(svc.Name)
	}
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

func promptIKEv2Input() (provision.Input, error) {
	var (
		display         string
		server          string
		remoteID        string
		localID         string
		username        string
		password        string
		onDemandEnabled bool
		matchDomainsCSV string
	)
	requiredText := func(label string) func(string) error {
		return func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("%s is required", label)
			}
			return nil
		}
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Display name").
				Description("Shown in System Settings and used as YAML system.display_name").
				Value(&display).
				Validate(requiredText("display name")),
			huh.NewInput().
				Title("IKEv2 server").
				Description("Hostname or IP").
				Value(&server).
				Validate(requiredText("server")),
			huh.NewInput().
				Title("Remote identifier").
				Description("Optional; defaults to the server hostname").
				Value(&remoteID),
			huh.NewInput().
				Title("Local identifier").
				Description("Optional; defaults to the EAP username").
				Value(&localID),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("EAP username").
				Value(&username).
				Validate(requiredText("username")),
			huh.NewInput().
				Title("EAP password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(requiredText("password")),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Connect On Demand?").
				Affirmative("Yes").
				Negative("No").
				Value(&onDemandEnabled),
			huh.NewInput().
				Title("Match domains").
				Description("Comma-separated list (only used when Connect On Demand is enabled)").
				Value(&matchDomainsCSV),
		),
	)
	if err := form.Run(); err != nil {
		return provision.Input{}, err
	}
	in := provision.Input{
		DisplayName:      strings.TrimSpace(display),
		Server:           strings.TrimSpace(server),
		RemoteIdentifier: strings.TrimSpace(remoteID),
		LocalIdentifier:  strings.TrimSpace(localID),
		Username:         strings.TrimSpace(username),
		Password:         password,
	}
	if onDemandEnabled {
		in.OnDemand = &provision.OnDemand{
			Enabled:      true,
			MatchDomains: splitTrimNonEmpty(matchDomainsCSV, ","),
		}
	}
	return in, nil
}

func splitTrimNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func promptMode() (bool, error) {
	var create bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("How do you want to add a profile?").
				Options(
					huh.NewOption("Register an existing system VPN", false),
					huh.NewOption("Create a new VPN from scratch (.mobileconfig)", true),
				).
				Value(&create),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return create, nil
}
