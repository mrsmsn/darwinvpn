package provision

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func validInput() Input {
	return Input{
		DisplayName: "Work VPN",
		Server:      "vpn.example.com",
		Username:    "user@example.com",
		Password:    "REDACTED",
	}
}

// fakeOpen captures the path passed to `open` so the test can assert it
// matches the temp file InstallAndDetect actually wrote.
type fakeOpen struct {
	called int32
	path   atomic.Value // string
	err    error
}

func (f *fakeOpen) hook(path string) error {
	atomic.StoreInt32(&f.called, 1)
	f.path.Store(path)
	return f.err
}

func swapOpen(t *testing.T, hook func(string) error) {
	t.Helper()
	prev := openCmd
	openCmd = hook
	t.Cleanup(func() { openCmd = prev })
}

func TestInstallAndDetect_ReturnsNewlyInstalledService(t *testing.T) {
	open := &fakeOpen{}
	swapOpen(t, open.hook)

	baseline := []vpn.Service{
		{UUID: "u-baseline", Name: "existing"},
	}
	var calls int32
	list := func(ctx context.Context) ([]vpn.Service, error) {
		c := atomic.AddInt32(&calls, 1)
		if c <= 1 {
			return baseline, nil
		}
		return append(baseline, vpn.Service{
			UUID: "u-new", Name: "Work VPN", Status: vpn.StatusDisconnected,
		}), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := InstallAndDetect(ctx, validInput(), list, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("InstallAndDetect: %v", err)
	}
	if got.UUID != "u-new" || got.Name != "Work VPN" {
		t.Errorf("got %+v, want UUID=u-new Name=Work VPN", got)
	}
	if atomic.LoadInt32(&open.called) != 1 {
		t.Error("open was not invoked")
	}
	path, _ := open.path.Load().(string)
	if path == "" {
		t.Error("open received empty path")
	}
	// Temp file must have been removed once InstallAndDetect returned.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("temp file %s should have been deleted, stat err=%v", path, err)
	}
}

func TestInstallAndDetect_PropagatesContextCancel(t *testing.T) {
	swapOpen(t, func(string) error { return nil })
	list := func(context.Context) ([]vpn.Service, error) {
		return []vpn.Service{{UUID: "u-baseline"}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := InstallAndDetect(ctx, validInput(), list, 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want context.DeadlineExceeded", err)
	}
}

func TestInstallAndDetect_OpenError(t *testing.T) {
	swapOpen(t, func(string) error { return errors.New("boom") })
	list := func(context.Context) ([]vpn.Service, error) {
		return nil, nil
	}
	_, err := InstallAndDetect(context.Background(), validInput(), list, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from open hook, got nil")
	}
}

func TestInstallAndDetect_ValidatesInputBeforeWriting(t *testing.T) {
	called := false
	swapOpen(t, func(string) error { called = true; return nil })
	_, err := InstallAndDetect(context.Background(), Input{}, func(context.Context) ([]vpn.Service, error) {
		return nil, nil
	}, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if called {
		t.Error("open should not have been called when input is invalid")
	}
}
