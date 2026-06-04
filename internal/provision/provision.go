// Package provision generates the .mobileconfig (Apple Configuration Profile)
// that "darwinvpn add" hands to System Settings for installation when the
// user opts into Mode B (interactive new VPN creation).
//
// Phase 2 supports IKEv2 with EAP authentication only (username + password).
// Certificate and shared-secret variants are tracked for a later phase
// per docs/pj.md §15.
//
// The bytes returned by Generate are an Apple XML plist suitable for
// writing to a 0600 temporary file and opening with `open <file>` so the
// user is prompted to approve the profile in System Settings.
package provision

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"howett.net/plist"
)

// Input collects the fields the user is asked for during interactive
// provisioning, before any of them touch a config file.
type Input struct {
	// DisplayName is the human-readable name shown in System Settings and
	// also used as the darwinvpn config's system.display_name.
	DisplayName string

	// Server is the IKEv2 endpoint (hostname or IP).
	Server string

	// RemoteIdentifier defaults to Server when empty.
	RemoteIdentifier string

	// LocalIdentifier identifies the client side (typically the EAP
	// username or an email address).
	LocalIdentifier string

	// Username and Password are the EAP credentials. They are written into
	// the mobileconfig in plaintext, which is why the generated file must
	// be 0600 and deleted right after install (docs/pj.md §9).
	Username string
	Password string

	// OnDemand, when non-nil and Enabled, adds OnDemandRules with the
	// given match_domains so macOS reconnects automatically when the user
	// accesses one of those hosts.
	OnDemand *OnDemand
}

// OnDemand mirrors the YAML-side ikev2.on_demand block.
type OnDemand struct {
	Enabled      bool
	MatchDomains []string
}

// Validate returns an error if Input is missing fields that the
// mobileconfig requires.
func (in Input) Validate() error {
	switch {
	case strings.TrimSpace(in.DisplayName) == "":
		return errors.New("provision: DisplayName is required")
	case strings.TrimSpace(in.Server) == "":
		return errors.New("provision: Server is required")
	case strings.TrimSpace(in.Username) == "":
		return errors.New("provision: Username is required (EAP)")
	case strings.TrimSpace(in.Password) == "":
		return errors.New("provision: Password is required (EAP)")
	}
	return nil
}

// Generate builds an Apple XML plist for an IKEv2 VPN configuration with EAP
// authentication.
func Generate(in Input) ([]byte, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	remoteID := in.RemoteIdentifier
	if remoteID == "" {
		remoteID = in.Server
	}
	localID := in.LocalIdentifier
	if localID == "" {
		localID = in.Username
	}

	payloadUUID, err := newUUID()
	if err != nil {
		return nil, err
	}
	configUUID, err := newUUID()
	if err != nil {
		return nil, err
	}

	ikev2 := ikev2Block{
		AuthenticationMethod: "None",
		ExtendedAuthEnabled:  1,
		RemoteAddress:        in.Server,
		RemoteIdentifier:     remoteID,
		LocalIdentifier:      localID,
		AuthName:             in.Username,
		AuthPassword:         in.Password,
		DisableMOBIKE:        0,
		DisableRedirect:      0,
		EnablePFS:            0,
		EnableRevocationCheck: 0,
	}

	vpn := vpnPayload{
		PayloadType:        "com.apple.vpn.managed",
		PayloadIdentifier:  "io.github.mrsmsn.darwinvpn.vpn." + payloadUUID,
		PayloadUUID:        payloadUUID,
		PayloadVersion:     1,
		PayloadDisplayName: in.DisplayName,
		UserDefinedName:    in.DisplayName,
		VPNType:            "IKEv2",
		IKEv2:              ikev2,
	}
	if in.OnDemand != nil && in.OnDemand.Enabled {
		vpn.OnDemandEnabled = 1
		vpn.OnDemandRules = onDemandRules(in.OnDemand.MatchDomains)
	}

	doc := configuration{
		PayloadContent:     []vpnPayload{vpn},
		PayloadDisplayName: in.DisplayName,
		PayloadIdentifier:  "io.github.mrsmsn.darwinvpn." + configUUID,
		PayloadUUID:        configUUID,
		PayloadType:        "Configuration",
		PayloadVersion:     1,
	}

	return plist.MarshalIndent(doc, plist.XMLFormat, "\t")
}

func onDemandRules(matchDomains []string) []onDemandRule {
	if len(matchDomains) == 0 {
		return nil
	}
	return []onDemandRule{
		{
			Action:           "Connect",
			URLStringProbe:   "",
			DNSDomainMatch:   matchDomains,
		},
	}
}

// newUUID generates an RFC 4122 v4 UUID without pulling in google/uuid.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08X-%04X-%04X-%04X-%012X",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
