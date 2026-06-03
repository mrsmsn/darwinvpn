// bridge.m — Objective-C implementation of the dvpn_* C ABI.
//
// Phase 1 Step 1 ships the skeleton: each function returns DVPN_ERR_INTERNAL
// so the Go side compiles and ErrUnsupported is replaced by a real (if not
// yet useful) Manager. Subsequent Phase 1 steps fill in the real bodies.

#include "bridge.h"

#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>

int dvpn_list(dvpn_service_t **services, int *count) {
    if (services) *services = NULL;
    if (count) *count = 0;
    return DVPN_OK;
}

int dvpn_start(const char *uuid) {
    (void)uuid;
    return DVPN_ERR_INTERNAL;
}

int dvpn_stop(const char *uuid) {
    (void)uuid;
    return DVPN_ERR_INTERNAL;
}

int dvpn_status(const char *uuid, int *status_out) {
    (void)uuid;
    if (status_out) *status_out = 0;
    return DVPN_ERR_INTERNAL;
}
