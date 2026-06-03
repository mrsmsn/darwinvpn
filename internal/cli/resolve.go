package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// errAmbiguous indicates the caller passed no name but the system has more
// than one IKEv2 profile, so there is no obvious default to act on.
var errAmbiguous = errors.New("multiple profiles registered; specify one by name")

// errNoProfiles indicates List() returned zero VPN profiles.
var errNoProfiles = errors.New("no VPN profile is registered on this system")

// resolveService picks the Service to operate on. When name is empty, the
// single registered profile is used; if more than one exists, errAmbiguous is
// returned. When name is non-empty, it is matched case-insensitively against
// Service.Name first, then Service.UUID. Phase 2 will layer YAML-based
// aliases on top via internal/config.
func resolveService(ctx context.Context, mgr vpn.Manager, name string) (vpn.Service, error) {
	services, err := mgr.List(ctx)
	if err != nil {
		return vpn.Service{}, err
	}
	if len(services) == 0 {
		return vpn.Service{}, errNoProfiles
	}
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
