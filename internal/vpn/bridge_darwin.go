//go:build darwin && cgo

package vpn

// NewDarwinManager returns a Manager backed by macOS private NetworkExtension
// APIs (NEConfigurationManager and ne_session_*). The implementation lives in
// a cgo Objective-C bridge to be added in Phase 1; see docs/pj.md §6.
//
// Phase 0 ships a stub that returns ErrUnsupported so the CLI can be wired
// end-to-end before the bridge is functional.
func NewDarwinManager() (Manager, error) {
	return nil, ErrUnsupported
}
