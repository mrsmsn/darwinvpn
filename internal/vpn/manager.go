// Package vpn provides an abstraction over macOS VPN session management.
//
// The Manager interface is implemented by both a cgo-backed bridge to Apple's
// private NetworkExtension APIs (bridge_darwin.go, populated in Phase 1) and
// an in-memory fake (fake.go) used by unit tests. See docs/pj.md §5 and §11
// for the layering rationale.
package vpn

import (
	"context"
	"errors"
)

// Status represents a VPN session state. The integer values are intentionally
// aligned with macOS's ne_session_status_t so the cgo bridge can pass values
// through without translation. The mapping was confirmed against a live
// runtime via docs/phase1-probe (see docs/pj.md §6.5).
//
// StatusUnknown is darwinvpn-specific (negative so it cannot collide with the
// kernel enum) and is used by the fake implementation and error paths.
type Status int

const (
	StatusInvalid       Status = 0
	StatusDisconnected  Status = 1
	StatusConnecting    Status = 2
	StatusConnected     Status = 3
	StatusReasserting   Status = 4
	StatusDisconnecting Status = 5
	StatusUnknown       Status = -1
)

func (s Status) String() string {
	switch s {
	case StatusInvalid:
		return "invalid"
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusReasserting:
		return "reasserting"
	case StatusDisconnecting:
		return "disconnecting"
	default:
		return "unknown"
	}
}

// Service represents a single VPN configuration enumerated from the system.
// It corresponds to one NEConfiguration entry on macOS.
type Service struct {
	UUID   string
	Name   string
	Status Status
}

// Manager abstracts VPN session control. Implementations must be safe for
// concurrent use by multiple goroutines.
type Manager interface {
	List(ctx context.Context) ([]Service, error)
	Start(ctx context.Context, uuid string) error
	Stop(ctx context.Context, uuid string) error
	Status(ctx context.Context, uuid string) (Status, error)
}

// Sentinel errors returned by Manager implementations. The bridge layer maps
// the underlying C ABI error codes to these.
var (
	ErrNotFound      = errors.New("vpn: service not found")
	ErrAlreadyActive = errors.New("vpn: session already started")
	ErrNotActive     = errors.New("vpn: session not started")
	ErrUnsupported   = errors.New("vpn: operation not supported on this platform")
)
