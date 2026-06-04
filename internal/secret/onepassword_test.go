package secret

import (
	"context"
	"strings"
	"testing"
)

func swapOp(t *testing.T, hook func(context.Context, ...string) ([]byte, error)) {
	t.Helper()
	prev := runOp
	runOp = hook
	t.Cleanup(func() { runOp = prev })
}

func TestOnePassword_GetSendsExpectedArgs(t *testing.T) {
	stub := &stubRun{stdout: stubLine("hunter2")}
	swapOp(t, stub.hook)
	got, err := NewOnePassword().Get(context.Background(), "op://Personal/Work GHE/password")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("value = %q, want hunter2", got)
	}
	if len(stub.args) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(stub.args))
	}
	expectArgs(t, stub.args[0], []string{"read", "op://Personal/Work GHE/password"})
}

func TestOnePassword_GetRejectsNonOpScheme(t *testing.T) {
	stub := &stubRun{}
	swapOp(t, stub.hook)
	_, err := NewOnePassword().Get(context.Background(), "darwinvpn/work-ghe")
	if err == nil || !strings.Contains(err.Error(), "must start with op://") {
		t.Errorf("got %v, want op:// validation error", err)
	}
	if len(stub.args) != 0 {
		t.Errorf("op should not be invoked on invalid ref, got %v", stub.args)
	}
}

func TestOnePassword_GetRefusesEmptyRef(t *testing.T) {
	_, err := NewOnePassword().Get(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "ref is empty") {
		t.Errorf("got %v, want empty-ref error", err)
	}
}

func TestOnePassword_SetReturnsHelpfulError(t *testing.T) {
	err := NewOnePassword().Set(context.Background(), "op://Personal/x/password", "v")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("got %v, want not-supported error", err)
	}
}
