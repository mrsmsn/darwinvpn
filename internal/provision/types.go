package provision

// configuration is the outer plist dictionary documented in Apple's
// "Configuration Profile Reference". Field tags use the howett.net/plist
// convention; the marshaller turns these into <key>...</key><value> pairs.
type configuration struct {
	PayloadContent     []vpnPayload `plist:"PayloadContent"`
	PayloadDisplayName string       `plist:"PayloadDisplayName"`
	PayloadIdentifier  string       `plist:"PayloadIdentifier"`
	PayloadUUID        string       `plist:"PayloadUUID"`
	PayloadType        string       `plist:"PayloadType"`
	PayloadVersion     int          `plist:"PayloadVersion"`
}

// vpnPayload is the inner com.apple.vpn.managed dictionary.
type vpnPayload struct {
	PayloadType        string     `plist:"PayloadType"`
	PayloadIdentifier  string     `plist:"PayloadIdentifier"`
	PayloadUUID        string     `plist:"PayloadUUID"`
	PayloadVersion     int        `plist:"PayloadVersion"`
	PayloadDisplayName string     `plist:"PayloadDisplayName"`
	UserDefinedName    string     `plist:"UserDefinedName"`
	VPNType            string     `plist:"VPNType"`
	IKEv2              ikev2Block `plist:"IKEv2"`

	// OnDemand* are omitted from the output when zero / nil.
	OnDemandEnabled int            `plist:"OnDemandEnabled,omitempty"`
	OnDemandRules   []onDemandRule `plist:"OnDemandRules,omitempty"`
}

// ikev2Block holds the EAP-only IKEv2 settings Phase 2 supports.
type ikev2Block struct {
	AuthenticationMethod  string `plist:"AuthenticationMethod"`
	ExtendedAuthEnabled   int    `plist:"ExtendedAuthEnabled"`
	RemoteAddress         string `plist:"RemoteAddress"`
	RemoteIdentifier      string `plist:"RemoteIdentifier"`
	LocalIdentifier       string `plist:"LocalIdentifier"`
	AuthName              string `plist:"AuthName"`
	AuthPassword          string `plist:"AuthPassword"`
	DisableMOBIKE         int    `plist:"DisableMOBIKE"`
	DisableRedirect       int    `plist:"DisableRedirect"`
	EnablePFS             int    `plist:"EnablePFS"`
	EnableRevocationCheck int    `plist:"EnableRevocationCheck"`
}

type onDemandRule struct {
	Action         string   `plist:"Action"`
	URLStringProbe string   `plist:"URLStringProbe,omitempty"`
	DNSDomainMatch []string `plist:"DNSDomainMatch,omitempty"`
}
