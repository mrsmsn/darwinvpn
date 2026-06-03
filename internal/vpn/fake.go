package vpn

import (
	"context"
	"sync"
)

// FakeManager is an in-memory Manager implementation for unit tests.
type FakeManager struct {
	mu       sync.Mutex
	services []*Service
}

// NewFakeManager constructs a FakeManager seeded with the given services.
// The seed slice is copied; subsequent mutations on the caller side do not
// affect the manager.
func NewFakeManager(initial []Service) *FakeManager {
	fm := &FakeManager{services: make([]*Service, 0, len(initial))}
	for i := range initial {
		s := initial[i]
		fm.services = append(fm.services, &s)
	}
	return fm
}

// findLocked must be called with f.mu held.
func (f *FakeManager) findLocked(uuid string) *Service {
	for _, s := range f.services {
		if s.UUID == uuid {
			return s
		}
	}
	return nil
}

func (f *FakeManager) List(ctx context.Context) ([]Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Service, 0, len(f.services))
	for _, s := range f.services {
		out = append(out, *s)
	}
	return out, nil
}

func (f *FakeManager) Start(ctx context.Context, uuid string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.findLocked(uuid)
	if s == nil {
		return ErrNotFound
	}
	if s.Status == StatusConnected || s.Status == StatusConnecting {
		return ErrAlreadyActive
	}
	s.Status = StatusConnected
	return nil
}

func (f *FakeManager) Stop(ctx context.Context, uuid string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.findLocked(uuid)
	if s == nil {
		return ErrNotFound
	}
	if s.Status == StatusDisconnected {
		return ErrNotActive
	}
	s.Status = StatusDisconnected
	return nil
}

func (f *FakeManager) Status(ctx context.Context, uuid string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return StatusUnknown, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.findLocked(uuid)
	if s == nil {
		return StatusUnknown, ErrNotFound
	}
	return s.Status, nil
}

// Compile-time interface assertion.
var _ Manager = (*FakeManager)(nil)
