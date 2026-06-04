package secret

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// stubRun records the args of each call and returns a fixed response.
type stubRun struct {
	args   [][]string
	stdout []byte
	err    error
}

func (s *stubRun) hook(_ context.Context, args ...string) ([]byte, error) {
	s.args = append(s.args, append([]string(nil), args...))
	return s.stdout, s.err
}

func swapSecurity(t *testing.T, hook func(context.Context, ...string) ([]byte, error)) {
	t.Helper()
	prev := runSecurity
	runSecurity = hook
	t.Cleanup(func() { runSecurity = prev })
}

func TestKeychain_GetSendsExpectedArgs(t *testing.T) {
	stub := &stubRun{stdout: stubLine("super-secret")}
	swapSecurity(t, stub.hook)
	got, err := NewKeychain().Get(context.Background(), "darwinvpn/work-ghe")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "super-secret" {
		t.Errorf("value = %q, want super-secret", got)
	}
	if len(stub.args) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(stub.args))
	}
	expectArgs(t, stub.args[0], []string{
		"find-generic-password", "-a", "darwinvpn", "-s", "darwinvpn/work-ghe", "-w",
	})
}

func TestKeychain_GetTranslatesNotFound(t *testing.T) {
	stub := &stubRun{err: fakeExitError(44)}
	swapSecurity(t, stub.hook)
	_, err := NewKeychain().Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want errors.Is ErrNotFound", err)
	}
}

func TestKeychain_GetRefusesEmptyRef(t *testing.T) {
	_, err := NewKeychain().Get(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "ref is empty") {
		t.Errorf("got %v, want empty-ref error", err)
	}
}

func TestKeychain_SetUpsertsViaSecurity(t *testing.T) {
	stub := &stubRun{}
	swapSecurity(t, stub.hook)
	if err := NewKeychain().Set(context.Background(), "darwinvpn/work-ghe", "hunter2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(stub.args) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(stub.args))
	}
	expectArgs(t, stub.args[0], []string{
		"add-generic-password", "-U", "-a", "darwinvpn",
		"-s", "darwinvpn/work-ghe", "-w", "hunter2",
	})
}

func TestKeychain_SetRefusesEmptyValue(t *testing.T) {
	stub := &stubRun{}
	swapSecurity(t, stub.hook)
	err := NewKeychain().Set(context.Background(), "darwinvpn/x", "")
	if err == nil || !strings.Contains(err.Error(), "refusing to store empty value") {
		t.Errorf("got %v, want empty-value error", err)
	}
	if len(stub.args) != 0 {
		t.Errorf("Set should not call security on empty value, got %v", stub.args)
	}
}

func TestFor_KnownProviders(t *testing.T) {
	if _, err := For("keychain"); err != nil {
		t.Errorf("For(keychain): %v", err)
	}
	if _, err := For("1password"); err != nil {
		t.Errorf("For(1password): %v", err)
	}
	if _, err := For("nosuch"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("For(nosuch): %v", err)
	}
	if _, err := For(""); !errors.Is(err, ErrUnsupported) {
		t.Errorf("For(empty): %v", err)
	}
}

// --- helpers ---

func stubLine(s string) []byte {
	return []byte(s + "\n")
}

func expectArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("argv len = %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// fakeExitError fabricates an *exec.ExitError with the desired exit code so
// the keychain Get path can map exit 44 -> ErrNotFound without spawning a
// real process.
func fakeExitError(code int) error {
	// Resolve a binary that exists everywhere a Go test runs and exits
	// with a controllable code via the shell.
	cmd := exec.Command("sh", "-c", "exit "+itoa(code))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func itoa(n int) string {
	// Minimal int->string to avoid pulling strconv into the helper.
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

