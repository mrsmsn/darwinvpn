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

// Status represents a VPN session state. The concrete integer values used by
// the underlying ne_session_status_t enum will be confirmed against a live
// macOS runtime in Phase 1 (docs/pj.md §6.5). For Phase 0, callers only need
// to compare against the named constants.
type Status int

const (
	StatusUnknown Status = iota
	StatusInvalid
	StatusDisconnected
	StatusConnecting
	StatusConnected
	StatusReasserting
	StatusDisconnecting
)

var statusNames = [...]string{
	"unknown",
	"invalid",
	"disconnected",
	"connecting",
	"connected",
	"reasserting",
	"disconnecting",
}

func (s Status) String() string {
	if s < 0 || int(s) >= len(statusNames) {
		return "unknown"
	}
	return statusNames[s]
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
