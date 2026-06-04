// Package secret resolves the credential references stored in
// config.Profile.Secret (docs/pj.md §8). Phase 2 supports the keychain
// provider; 1password lands in Step 6.
//
// Plaintext secrets never touch the YAML config — only a {provider, ref}
// pair is stored, and resolution is delayed until start (or until the user
// chooses to mirror a freshly entered password into the keychain during
// `darwinvpn add --create`).
package secret

import (
	"context"
	"errors"
	"fmt"
)

// Provider is the surface every backend implements.
type Provider interface {
	// Name returns the YAML provider identifier ("keychain", "1password").
	Name() string

	// Get returns the secret value for the given reference.
	Get(ctx context.Context, ref string) (string, error)

	// Set stores value under ref. Backends may upsert (Keychain does).
	Set(ctx context.Context, ref, value string) error
}

// ErrNotFound indicates the reference does not exist in the backend.
var ErrNotFound = errors.New("secret: reference not found")

// ErrUnsupported is returned by For when a config.Profile points at a
// provider this build doesn't know about.
var ErrUnsupported = errors.New("secret: unsupported provider")

// For returns the Provider implementation named by provider.
func For(provider string) (Provider, error) {
	switch provider {
	case "keychain":
		return NewKeychain(), nil
	case "1password":
		return nil, fmt.Errorf("%w: 1password lands in Phase 2 Step 6", ErrUnsupported)
	case "":
		return nil, fmt.Errorf("%w: provider is empty", ErrUnsupported)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupported, provider)
	}
}
