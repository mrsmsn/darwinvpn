package provision_test

import (
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/mrsmsn/darwinvpn/internal/provision"
)

func TestGenerate_ContainsRequiredKeys(t *testing.T) {
	in := provision.Input{
		DisplayName:      "Work VPN",
		Server:           "vpn.example.com",
		RemoteIdentifier: "vpn.example.com",
		LocalIdentifier:  "user@example.com",
		Username:         "user@example.com",
		Password:         "hunter2",
	}
	out, err := provision.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		"com.apple.vpn.managed",
		"<string>IKEv2</string>",
		"<string>vpn.example.com</string>",
		"<string>user@example.com</string>",
		"<string>hunter2</string>",
		"<string>Work VPN</string>",
		"<string>None</string>", // EAP via AuthenticationMethod=None + ExtendedAuthEnabled=1
		"ExtendedAuthEnabled",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

func TestGenerate_RoundTripsThroughPlistDecoder(t *testing.T) {
	in := provision.Input{
		DisplayName: "Work VPN",
		Server:      "vpn.example.com",
		Username:    "user@example.com",
		Password:    "hunter2",
	}
	out, err := provision.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var doc map[string]any
	if _, err := plist.Unmarshal(out, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, ok := doc["PayloadContent"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("PayloadContent shape unexpected: %T %v", doc["PayloadContent"], doc["PayloadContent"])
	}
	vpn := content[0].(map[string]any)
	if vpn["VPNType"] != "IKEv2" {
		t.Errorf("VPNType = %v, want IKEv2", vpn["VPNType"])
	}
	ikev2 := vpn["IKEv2"].(map[string]any)
	if ikev2["RemoteAddress"] != "vpn.example.com" {
		t.Errorf("RemoteAddress = %v", ikev2["RemoteAddress"])
	}
	if ikev2["AuthName"] != "user@example.com" {
		t.Errorf("AuthName = %v", ikev2["AuthName"])
	}
}

func TestGenerate_OnDemandRulesIncluded(t *testing.T) {
	in := provision.Input{
		DisplayName: "Work VPN",
		Server:      "vpn.example.com",
		Username:    "user@example.com",
		Password:    "hunter2",
		OnDemand: &provision.OnDemand{
			Enabled:      true,
			MatchDomains: []string{"ghe.example.com", "internal.example.com"},
		},
	}
	out, err := provision.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		"OnDemandEnabled",
		"OnDemandRules",
		"ghe.example.com",
		"internal.example.com",
		"<string>Connect</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("OnDemand output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

func TestGenerate_OnDemandSkippedWhenDisabled(t *testing.T) {
	in := provision.Input{
		DisplayName: "Work VPN",
		Server:      "vpn.example.com",
		Username:    "user@example.com",
		Password:    "hunter2",
	}
	out, err := provision.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(string(out), "OnDemandEnabled") {
		t.Errorf("OnDemandEnabled present even though OnDemand was nil")
	}
}

func TestGenerate_RequiresRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		in   provision.Input
		want string
	}{
		{"missing display", provision.Input{Server: "s", Username: "u", Password: "p"}, "DisplayName"},
		{"missing server", provision.Input{DisplayName: "d", Username: "u", Password: "p"}, "Server"},
		{"missing user", provision.Input{DisplayName: "d", Server: "s", Password: "p"}, "Username"},
		{"missing pass", provision.Input{DisplayName: "d", Server: "s", Username: "u"}, "Password"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := provision.Generate(c.in)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want error mentioning %s", err, c.want)
			}
		})
	}
}

func TestGenerate_RemoteIDDefaultsToServer(t *testing.T) {
	in := provision.Input{
		DisplayName: "Work VPN",
		Server:      "vpn.example.com",
		// RemoteIdentifier omitted on purpose.
		Username: "user@example.com",
		Password: "hunter2",
	}
	out, _ := provision.Generate(in)
	if !strings.Contains(string(out), "<key>RemoteIdentifier</key>\n\t\t\t\t<string>vpn.example.com</string>") &&
		!strings.Contains(string(out), "RemoteIdentifier") {
		t.Error("RemoteIdentifier missing or not derived from Server")
	}
}
