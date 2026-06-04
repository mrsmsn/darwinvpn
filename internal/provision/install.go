package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

// ListFunc is the polling source InstallAndDetect calls to find the newly
// installed NEConfiguration. In production this is vpn.Manager.List wrapped
// to drop ctx through; tests pass a closure that returns canned results.
type ListFunc func(ctx context.Context) ([]vpn.Service, error)

// openCmd is overridden by tests to avoid actually launching System
// Settings during a unit test run.
var openCmd = func(path string) error {
	return exec.Command("open", path).Run()
}

// InstallAndDetect drives Mode B end-to-end:
//
//  1. Generate the Apple XML plist from in.
//  2. Write it to a 0600 temp file (deleted before this function returns).
//  3. Snapshot the current VPN list so we know what was there before.
//  4. Open the file so System Settings prompts for approval.
//  5. Poll list every `every` until a UUID appears that wasn't in the
//     snapshot, or until ctx is canceled.
//
// The returned vpn.Service points at the freshly installed configuration;
// callers feed its UUID and Name into config.System.
func InstallAndDetect(ctx context.Context, in Input, list ListFunc, every time.Duration) (vpn.Service, error) {
	data, err := Generate(in)
	if err != nil {
		return vpn.Service{}, err
	}
	path, err := writeTempFile(data)
	if err != nil {
		return vpn.Service{}, err
	}
	defer os.Remove(path)

	baseline, err := list(ctx)
	if err != nil {
		return vpn.Service{}, err
	}
	seen := make(map[string]struct{}, len(baseline))
	for _, s := range baseline {
		seen[s.UUID] = struct{}{}
	}

	if err := openCmd(path); err != nil {
		return vpn.Service{}, fmt.Errorf("open %s: %w", path, err)
	}

	if every <= 0 {
		every = 2 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return vpn.Service{}, ctx.Err()
		case <-t.C:
			current, err := list(ctx)
			if err != nil {
				return vpn.Service{}, err
			}
			for _, s := range current {
				if _, ok := seen[s.UUID]; !ok {
					return s, nil
				}
			}
		}
	}
}

// writeTempFile writes data to a fresh 0600 temp file under the system temp
// directory. The path is returned; callers (or the defer in
// InstallAndDetect) must remove it.
func writeTempFile(data []byte) (string, error) {
	f, err := os.CreateTemp("", "darwinvpn-*.mobileconfig")
	if err != nil {
		return "", err
	}
	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	if _, err := f.Write(data); err != nil {
		cleanup()
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// ErrDetectTimeout is returned via ctx.Err() wrapping when the user took too
// long approving the profile (or denied it altogether).
var ErrDetectTimeout = errors.New("provision: timed out waiting for the mobileconfig to be installed")
