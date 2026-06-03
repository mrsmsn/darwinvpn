package vpn_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mrsmsn/darwinvpn/internal/vpn"
)

func newFake(t *testing.T, services ...vpn.Service) *vpn.FakeManager {
	t.Helper()
	return vpn.NewFakeManager(services)
}

func TestList_Empty(t *testing.T) {
	fm := newFake(t)
	got, err := fm.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(got))
	}
}

func TestList_ReturnsAllServices(t *testing.T) {
	initial := []vpn.Service{
		{UUID: "uuid-a", Name: "alpha", Status: vpn.StatusDisconnected},
		{UUID: "uuid-b", Name: "beta", Status: vpn.StatusConnected},
	}
	fm := vpn.NewFakeManager(initial)
	got, err := fm.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0] != initial[0] || got[1] != initial[1] {
		t.Fatalf("got = %+v, want %+v", got, initial)
	}
}

func TestStart_TransitionsToConnected(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Name: "n", Status: vpn.StatusDisconnected})
	if err := fm.Start(context.Background(), "u"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st, err := fm.Status(context.Background(), "u")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != vpn.StatusConnected {
		t.Fatalf("got %v, want %v", st, vpn.StatusConnected)
	}
}

func TestStart_UnknownUUID_ReturnsErrNotFound(t *testing.T) {
	fm := newFake(t)
	err := fm.Start(context.Background(), "no-such")
	if !errors.Is(err, vpn.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestStart_AlreadyConnected_ReturnsErrAlreadyActive(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Status: vpn.StatusConnected})
	err := fm.Start(context.Background(), "u")
	if !errors.Is(err, vpn.ErrAlreadyActive) {
		t.Fatalf("got %v, want ErrAlreadyActive", err)
	}
}

func TestStop_TransitionsToDisconnected(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Status: vpn.StatusConnected})
	if err := fm.Stop(context.Background(), "u"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, err := fm.Status(context.Background(), "u")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != vpn.StatusDisconnected {
		t.Fatalf("got %v, want StatusDisconnected", st)
	}
}

func TestStop_UnknownUUID_ReturnsErrNotFound(t *testing.T) {
	fm := newFake(t)
	err := fm.Stop(context.Background(), "no-such")
	if !errors.Is(err, vpn.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestStop_AlreadyDisconnected_ReturnsErrNotActive(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Status: vpn.StatusDisconnected})
	err := fm.Stop(context.Background(), "u")
	if !errors.Is(err, vpn.ErrNotActive) {
		t.Fatalf("got %v, want ErrNotActive", err)
	}
}

func TestStatus_UnknownUUID_ReturnsErrNotFound(t *testing.T) {
	fm := newFake(t)
	_, err := fm.Status(context.Background(), "no-such")
	if !errors.Is(err, vpn.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestStatus_ReflectsLatestState(t *testing.T) {
	ctx := context.Background()
	fm := newFake(t, vpn.Service{UUID: "u", Status: vpn.StatusDisconnected})
	if err := fm.Start(ctx, "u"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st, _ := fm.Status(ctx, "u"); st != vpn.StatusConnected {
		t.Fatalf("after Start: got %v, want Connected", st)
	}
	if err := fm.Stop(ctx, "u"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if st, _ := fm.Status(ctx, "u"); st != vpn.StatusDisconnected {
		t.Fatalf("after Stop: got %v, want Disconnected", st)
	}
}

func TestList_ReflectsStateChanges(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Name: "n", Status: vpn.StatusDisconnected})
	if err := fm.Start(context.Background(), "u"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, _ := fm.List(context.Background())
	if got[0].Status != vpn.StatusConnected {
		t.Fatalf("got %v, want Connected", got[0].Status)
	}
}

func TestConcurrentStartStop(t *testing.T) {
	initial := []vpn.Service{
		{UUID: "u1", Status: vpn.StatusDisconnected},
		{UUID: "u2", Status: vpn.StatusConnected},
	}
	fm := vpn.NewFakeManager(initial)
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = fm.Start(ctx, "u1")
			_ = fm.Stop(ctx, "u1")
		}()
		go func() {
			defer wg.Done()
			_ = fm.Stop(ctx, "u2")
			_ = fm.Start(ctx, "u2")
		}()
	}
	wg.Wait()
}

func TestContextCancellation(t *testing.T) {
	fm := newFake(t, vpn.Service{UUID: "u", Status: vpn.StatusDisconnected})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fm.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List: got %v, want context.Canceled", err)
	}
	if err := fm.Start(ctx, "u"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start: got %v, want context.Canceled", err)
	}
}

func TestStatusString(t *testing.T) {
	cases := []struct {
		s    vpn.Status
		want string
	}{
		{vpn.StatusUnknown, "unknown"},
		{vpn.StatusDisconnected, "disconnected"},
		{vpn.StatusConnecting, "connecting"},
		{vpn.StatusConnected, "connected"},
		{vpn.Status(999), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Status(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}
