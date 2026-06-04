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

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    char name[256];   // NEConfiguration.name (UTF-8, NUL-terminated, truncated)
    char uuid[40];    // NEConfiguration.identifier as UUID string
    int  status;      // ne_session_status_t value
} dvpn_service_t;

// dvpn_status_detail_t carries the rich status returned by dvpn_status_detail.
// Strings are UTF-8 NUL-terminated and silently truncated to fit the buffer.
// Empty strings mean the underlying NetworkExtension property is nil or the
// protocol is not NEVPNProtocolIKEv2. connected_at_unix is 0 when the session
// is not connected or the timestamp could not be retrieved.
typedef struct {
    int     status;
    char    server_address[256];
    char    remote_identifier[256];
    char    username[256];
    char    ipv4_address[64];   // primary IPv4 bound to the tunnel interface
    char    ipv6_address[64];   // primary IPv6 bound to the tunnel interface
    int64_t connected_at_unix;
} dvpn_status_detail_t;

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

// Read the rich status (state plus IKEv2 protocol fields) for uuid into *out.
// *out is zeroed on entry. Missing protocol fields are returned as empty
// strings rather than as an error.
int dvpn_status_detail(const char *uuid, dvpn_status_detail_t *out);

#ifdef __cplusplus
}
#endif

#endif // DVPN_BRIDGE_H
