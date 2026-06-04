package secret

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// runOp is overridden in tests to avoid hitting the real `op` CLI.
var runOp = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "op", args...).Output()
}

// onePassword resolves "op://Vault/Item/field" references through the
// 1Password CLI. Set is intentionally unimplemented: 1Password item layout
// (vault, category, fields) is too variable for a generic upsert, so
// darwinvpn expects the user to manage the item in 1Password themselves and
// just record a ref in the YAML profile.
type onePassword struct{}

// NewOnePassword returns a Provider that delegates to the `op` CLI.
func NewOnePassword() Provider { return onePassword{} }

func (onePassword) Name() string { return "1password" }

func (onePassword) Get(ctx context.Context, ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("secret/1password: ref is empty")
	}
	if !strings.HasPrefix(ref, "op://") {
		return "", fmt.Errorf("secret/1password: ref %q must start with op://", ref)
	}
	out, err := runOp(ctx, "read", ref)
	if err != nil {
		// Surface the most common failure (no signed-in account) with a
		// clearer message than the raw exec error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("secret/1password: op read %s: %s",
				ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("secret/1password: op read %s: %w", ref, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (onePassword) Set(_ context.Context, _, _ string) error {
	return fmt.Errorf("secret/1password: Set is not supported; manage the item directly in 1Password and reference it via op://Vault/Item/field in config.yaml")
}
