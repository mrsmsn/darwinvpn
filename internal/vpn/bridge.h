// bridge.h — C ABI surfaced from bridge.m to bridge_darwin.go.
//
// The Objective-C / cgo split follows docs/pj.md §5 / §6.4: NEConfigurationManager
// enumeration and ne_session_* calls live in bridge.m; the Go side only sees
// this small, synchronous C ABI.
//
// Return convention: 0 = success, non-zero = error code (mapped to a Go
// sentinel error by bridge_darwin.go).

#ifndef DVPN_BRIDGE_H
#define DVPN_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    char name[256];   // NEConfiguration.name (UTF-8, NUL-terminated, truncated)
    char uuid[40];    // NEConfiguration.identifier as UUID string
    int  status;      // ne_session_status_t value
} dvpn_service_t;

// Error codes returned by the bridge.
#define DVPN_OK              0
#define DVPN_ERR_INTERNAL    1   // unspecified failure (NSError, exception, etc.)
#define DVPN_ERR_NOT_FOUND   2   // no NEConfiguration matched the supplied UUID
#define DVPN_ERR_TIMEOUT     3   // async API did not complete within the wait
#define DVPN_ERR_ALREADY     4   // start on an already-active session
#define DVPN_ERR_NOT_ACTIVE  5   // stop on an inactive session

// Enumerate IKEv2 Personal VPN configurations. Caller owns *services and must
// release with free(); pass NULL when count == 0.
int dvpn_list(dvpn_service_t **services, int *count);

// Start the VPN session identified by uuid (NUL-terminated UUID string).
int dvpn_start(const char *uuid);

// Stop the VPN session identified by uuid.
int dvpn_stop(const char *uuid);

// Read the current ne_session_status_t for uuid into *status_out.
int dvpn_status(const char *uuid, int *status_out);

#ifdef __cplusplus
}
#endif

#endif // DVPN_BRIDGE_H
