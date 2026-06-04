//go:build darwin && cgo

package vpn

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework NetworkExtension
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"fmt"
	"time"
	"unsafe"
)

type darwinManager struct{}

// NewDarwinManager returns a Manager backed by macOS NetworkExtension private
// APIs via the cgo bridge in bridge.m.
func NewDarwinManager() (Manager, error) {
	return &darwinManager{}, nil
}

func (d *darwinManager) List(ctx context.Context) ([]Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var cServices *C.dvpn_service_t
	var count C.int
	if rc := C.dvpn_list(&cServices, &count); rc != C.DVPN_OK {
		return nil, mapBridgeError(int(rc), "dvpn_list")
	}
	if cServices != nil {
		defer C.free(unsafe.Pointer(cServices))
	}
	n := int(count)
	if n == 0 {
		return []Service{}, nil
	}
	out := make([]Service, n)
	arr := unsafe.Slice(cServices, n)
	for i := 0; i < n; i++ {
		out[i] = Service{
			Name:   C.GoString(&arr[i].name[0]),
			UUID:   C.GoString(&arr[i].uuid[0]),
			Status: Status(int(arr[i].status)),
		}
	}
	return out, nil
}

func (d *darwinManager) Start(ctx context.Context, uuid string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cUUID := C.CString(uuid)
	defer C.free(unsafe.Pointer(cUUID))
	if rc := C.dvpn_start(cUUID); rc != C.DVPN_OK {
		return mapBridgeError(int(rc), "dvpn_start")
	}
	return nil
}

func (d *darwinManager) Stop(ctx context.Context, uuid string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cUUID := C.CString(uuid)
	defer C.free(unsafe.Pointer(cUUID))
	if rc := C.dvpn_stop(cUUID); rc != C.DVPN_OK {
		return mapBridgeError(int(rc), "dvpn_stop")
	}
	return nil
}

func (d *darwinManager) Status(ctx context.Context, uuid string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return StatusUnknown, err
	}
	cUUID := C.CString(uuid)
	defer C.free(unsafe.Pointer(cUUID))
	var st C.int
	if rc := C.dvpn_status(cUUID, &st); rc != C.DVPN_OK {
		return StatusUnknown, mapBridgeError(int(rc), "dvpn_status")
	}
	return Status(int(st)), nil
}

func (d *darwinManager) StatusDetail(ctx context.Context, uuid string) (StatusDetail, error) {
	if err := ctx.Err(); err != nil {
		return StatusDetail{}, err
	}
	cUUID := C.CString(uuid)
	defer C.free(unsafe.Pointer(cUUID))
	var c C.dvpn_status_detail_t
	if rc := C.dvpn_status_detail(cUUID, &c); rc != C.DVPN_OK {
		return StatusDetail{}, mapBridgeError(int(rc), "dvpn_status_detail")
	}
	sd := StatusDetail{
		Status:           Status(int(c.status)),
		ServerAddress:    C.GoString(&c.server_address[0]),
		RemoteIdentifier: C.GoString(&c.remote_identifier[0]),
		Username:         C.GoString(&c.username[0]),
		IPv4Address:      C.GoString(&c.ipv4_address[0]),
		IPv6Address:      C.GoString(&c.ipv6_address[0]),
	}
	if c.connected_at_unix > 0 {
		sd.ConnectedAt = time.Unix(int64(c.connected_at_unix), 0).UTC()
	}
	return sd, nil
}

// mapBridgeError converts a C ABI error code into a Manager-layer sentinel
// where one exists. The call site name is preserved with %w for context.
func mapBridgeError(rc int, call string) error {
	switch rc {
	case int(C.DVPN_ERR_NOT_FOUND):
		return fmt.Errorf("%s: %w", call, ErrNotFound)
	case int(C.DVPN_ERR_ALREADY):
		return fmt.Errorf("%s: %w", call, ErrAlreadyActive)
	case int(C.DVPN_ERR_NOT_ACTIVE):
		return fmt.Errorf("%s: %w", call, ErrNotActive)
	default:
		return fmt.Errorf("%s: bridge returned %d", call, rc)
	}
}

var _ Manager = (*darwinManager)(nil)
