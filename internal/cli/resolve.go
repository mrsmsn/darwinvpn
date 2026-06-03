package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrsmsn/darwinvpn/internal/config"
	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// errAmbiguous indicates the caller passed no name but the system has more
// than one VPN with no default configured, so there is no obvious target.
var errAmbiguous = errors.New("multiple profiles registered; specify one by name")

// errNoProfiles indicates List() returned zero VPN profiles.
var errNoProfiles = errors.New("no VPN profile is registered on this system")

// resolveService picks the Service to operate on.
//
// Resolution order:
//  1. If a config file is loadable, look up name in it (or fall back to the
//     "default" profile when name is empty) and match against system.uuid
//     then system.display_name.
//  2. If config lookup fails (or no config exists), fall back to matching
//     name directly against Service.Name / Service.UUID. An empty name with
//     a single registered VPN resolves to that VPN.
func resolveService(cmd *cobra.Command, mgr vpn.Manager, name string) (vpn.Service, error) {
	services, err := mgr.List(cmd.Context())
	if err != nil {
		return vpn.Service{}, err
	}
	if len(services) == 0 {
		return vpn.Service{}, errNoProfiles
	}

	if svc, ok, err := resolveViaConfig(cmd, services, name); err != nil {
		return vpn.Service{}, err
	} else if ok {
		return svc, nil
	}

	return resolveDirect(services, name)
}

func resolveViaConfig(cmd *cobra.Command, services []vpn.Service, name string) (vpn.Service, bool, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPath)
	if errors.Is(err, config.ErrNotFound) {
		return vpn.Service{}, false, nil
	}
	if err != nil {
		return vpn.Service{}, false, err
	}
	profile, err := cfg.ProfileByName(name)
	if err != nil {
		// When a name was provided but the config doesn't recognise it,
		// silently fall through so direct UUID / display_name lookups
		// still work. Surface the config error only when the user asked
		// for "the default" but none is configured.
		if name == "" {
			return vpn.Service{}, false, err
		}
		return vpn.Service{}, false, nil
	}
	svc, err := matchProfile(services, profile)
	if err != nil {
		return vpn.Service{}, false, err
	}
	return svc, true, nil
}

func matchProfile(services []vpn.Service, p *config.Profile) (vpn.Service, error) {
	if p.System.UUID != "" {
		for _, s := range services {
			if strings.EqualFold(s.UUID, p.System.UUID) {
				return s, nil
			}
		}
	}
	if p.System.DisplayName != "" {
		for _, s := range services {
			if strings.EqualFold(s.Name, p.System.DisplayName) {
				return s, nil
			}
		}
	}
	return vpn.Service{}, fmt.Errorf("profile %q points to a system VPN that is not installed", p.Name)
}

func resolveDirect(services []vpn.Service, name string) (vpn.Service, error) {
	if name == "" {
		if len(services) > 1 {
			return vpn.Service{}, errAmbiguous
		}
		return services[0], nil
	}
	needle := strings.ToLower(name)
	for _, s := range services {
		if strings.ToLower(s.Name) == needle || strings.ToLower(s.UUID) == needle {
			return s, nil
		}
	}
	return vpn.Service{}, fmt.Errorf("profile %q not found", name)
}

// Compile-time assertion that context is still on the import surface (the
// migration above removed the direct ctx parameter in favour of cobra's
// Context()).
var _ = context.Background
