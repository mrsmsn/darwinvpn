package secret

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// keychainAccount is the value passed to `security` -a; using a constant
// account name keeps darwinvpn's references namespaced and easy to audit
// with `security dump-keychain | grep darwinvpn`.
const keychainAccount = "darwinvpn"

// runSecurity is overridden in tests to avoid hitting the real `security`
// CLI. It must behave like exec.CommandContext.Output.
var runSecurity = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "security", args...).Output()
}

// keychain is the macOS-native Provider.
type keychain struct{}

// NewKeychain returns a Provider backed by `security find-generic-password`
// and `security add-generic-password`. macOS prompts the user on first
// access if the keychain item's ACL hasn't whitelisted darwinvpn.
func NewKeychain() Provider { return keychain{} }

func (keychain) Name() string { return "keychain" }

func (keychain) Get(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("secret/keychain: ref is empty")
	}
	out, err := runSecurity(ctx, "find-generic-password",
		"-a", keychainAccount,
		"-s", ref,
		"-w",
	)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
			// `security` exits 44 ("SecKeychainSearchCopyNext: The
			// specified item could not be found in the keychain.")
			return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
		}
		return "", fmt.Errorf("secret/keychain: find %q: %w", ref, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (keychain) Set(ctx context.Context, ref, value string) error {
	if strings.TrimSpace(ref) == "" {
		return errors.New("secret/keychain: ref is empty")
	}
	if value == "" {
		return errors.New("secret/keychain: refusing to store empty value")
	}
	// -U upserts; -a/-s match the Get convention; -w supplies the secret.
	if _, err := runSecurity(ctx, "add-generic-password",
		"-U",
		"-a", keychainAccount,
		"-s", ref,
		"-w", value,
	); err != nil {
		return fmt.Errorf("secret/keychain: add %q: %w", ref, err)
	}
	return nil
}
