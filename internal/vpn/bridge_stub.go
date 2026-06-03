//go:build !darwin || !cgo

package vpn

// NewDarwinManager always returns ErrUnsupported on non-darwin platforms or
// when cgo is disabled. macOS is the only supported runtime target; this stub
// exists so go vet and go test pass on developer machines (e.g. Linux) that
// cross-compile or skip cgo.
func NewDarwinManager() (Manager, error) {
	return nil, ErrUnsupported
}
