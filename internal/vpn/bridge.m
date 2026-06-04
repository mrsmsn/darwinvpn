// bridge.m — Objective-C implementation of the dvpn_* C ABI declared in
// bridge.h. See docs/pj.md §6 for the architectural background and
// docs/phase1-probe/ for the runtime verification that motivates the IKEv2
// filter and the enum mapping used below.

#include "bridge.h"

#import <Foundation/Foundation.h>
#import <NetworkExtension/NetworkExtension.h>
#import <dispatch/dispatch.h>
#import <string.h>
#import <stdlib.h>
#import <xpc/xpc.h>

// ---- Private declarations (Timac/VPNStatus, blog.timac.org) -----------------

typedef struct ne_session_s *ne_session_t;

extern ne_session_t ne_session_create(uuid_t serviceID, int sessionConfigType);
extern void ne_session_release(ne_session_t session);

typedef void (^ne_session_status_block)(int status);
extern void ne_session_get_status(ne_session_t session,
                                  dispatch_queue_t queue,
                                  ne_session_status_block block);

extern void ne_session_start(ne_session_t session);
extern void ne_session_stop(ne_session_t session);

// ne_session_get_info returns an XPC dictionary describing the current
// session state. kind=2 yields the dictionary that contains
// "LastStatusChangeTime" (an XPC date == NSDate epoch), which is the
// timestamp of the most recent state transition — i.e. when the session
// entered Connected for a currently-connected VPN. This matches
// NEVPNConnection.connectedDate's semantics for our purposes. Confirmed
// against macOS 26.x via docs/phase1-probe/probe3.m.
typedef void (^ne_session_info_block)(xpc_object_t info);
extern void ne_session_get_info(ne_session_t session,
                                int kind,
                                dispatch_queue_t queue,
                                ne_session_info_block block);
#define NE_SESSION_INFO_KIND_STATE 2

#define NESESSION_TYPE_VPN 1

// NEConfigurationManager and NEConfiguration are private; declared here so
// the compiler is happy with the selector calls.

@interface NEConfiguration : NSObject
@property (readonly) NSUUID *identifier;
@property (readonly) NSString *name;
@property (readonly) id VPN;
@end

@interface NEConfigurationManager : NSObject
+ (instancetype)sharedManager;
- (void)loadConfigurationsWithCompletionQueue:(dispatch_queue_t)queue
                                      handler:(void (^)(NSArray<NEConfiguration *> *, NSError *))handler;
@end

// ---- Internal helpers -------------------------------------------------------

static dispatch_queue_t bridge_queue(void) {
    static dispatch_queue_t q;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        q = dispatch_queue_create("io.github.mrsmsn.darwinvpn.bridge",
                                  DISPATCH_QUEUE_SERIAL);
    });
    return q;
}

// Load all NEConfigurations synchronously. Returns nil on failure.
static NSArray *load_configurations(int *err_out) {
    @autoreleasepool {
        Class mgrClass = NSClassFromString(@"NEConfigurationManager");
        if (!mgrClass) {
            if (err_out) *err_out = DVPN_ERR_INTERNAL;
            return nil;
        }
        id mgr = [mgrClass performSelector:@selector(sharedManager)];
        if (!mgr) {
            if (err_out) *err_out = DVPN_ERR_INTERNAL;
            return nil;
        }

        SEL loadSel = NSSelectorFromString(
            @"loadConfigurationsWithCompletionQueue:handler:");
        if (![mgr respondsToSelector:loadSel]) {
            if (err_out) *err_out = DVPN_ERR_INTERNAL;
            return nil;
        }

        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block NSArray *result = nil;
        __block NSError *loadError = nil;

        void (*loadFn)(id, SEL, dispatch_queue_t, id) =
            (void (*)(id, SEL, dispatch_queue_t, id))
                [mgr methodForSelector:loadSel];
        loadFn(mgr, loadSel, bridge_queue(),
               ^(NSArray *configs, NSError *err) {
                   result = configs;
                   loadError = err;
                   dispatch_semaphore_signal(sem);
               });

        if (dispatch_semaphore_wait(sem,
                                    dispatch_time(DISPATCH_TIME_NOW,
                                                  10 * NSEC_PER_SEC)) != 0) {
            if (err_out) *err_out = DVPN_ERR_TIMEOUT;
            return nil;
        }
        if (loadError || !result) {
            if (err_out) *err_out = DVPN_ERR_INTERNAL;
            return nil;
        }
        return result;
    }
}

// is_vpn returns YES when configuration represents a VPN-flavoured
// NEConfiguration (anything with a non-nil VPN payload). The probe
// (docs/phase1-probe) showed NEConfigurationManager also returns
// application-firewall and Network Privacy entries; those have VPN == nil
// and are filtered here. probe2 then showed that both NEVPNProtocolIKEv2
// (Personal VPN) and NETunnelProviderProtocol (Tailscale et al.) return
// the correct status when probed with ne_session_create(uuid,
// NESessionTypeVPN=1), so no further protocol-class restriction is
// required to match scutil --nc list's coverage plus IKEv2.
static BOOL is_vpn(id configuration) {
    if (!configuration) return NO;
    id vpn = nil;
    @try {
        vpn = [configuration performSelector:@selector(VPN)];
    } @catch (NSException *e) {
        return NO;
    }
    return vpn != nil;
}

// find_configuration returns the NEConfiguration whose identifier UUIDString
// matches uuid_cstr (case-insensitive). Returns nil if not found.
static id find_configuration(const char *uuid_cstr, int *err_out) {
    if (!uuid_cstr) {
        if (err_out) *err_out = DVPN_ERR_INTERNAL;
        return nil;
    }
    NSArray *configurations = load_configurations(err_out);
    if (!configurations) return nil;

    NSString *needle = [[NSString stringWithUTF8String:uuid_cstr] uppercaseString];
    for (id cfg in configurations) {
        NSUUID *uid = [cfg performSelector:@selector(identifier)];
        if (!uid) continue;
        if ([[[uid UUIDString] uppercaseString] isEqualToString:needle]) {
            return cfg;
        }
    }
    if (err_out) *err_out = DVPN_ERR_NOT_FOUND;
    return nil;
}

// fetch_status synchronously reads ne_session_get_status for the given UUID.
static int fetch_status(NSUUID *uid, int *status_out) {
    uuid_t raw;
    [uid getUUIDBytes:raw];

    ne_session_t sess = ne_session_create(raw, NESESSION_TYPE_VPN);
    if (!sess) return DVPN_ERR_INTERNAL;

    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block int observed = 0;
    ne_session_get_status(sess, bridge_queue(), ^(int status) {
        observed = status;
        dispatch_semaphore_signal(sem);
    });

    int rc = DVPN_OK;
    if (dispatch_semaphore_wait(sem,
                                dispatch_time(DISPATCH_TIME_NOW,
                                              5 * NSEC_PER_SEC)) != 0) {
        rc = DVPN_ERR_TIMEOUT;
    } else if (status_out) {
        *status_out = observed;
    }
    ne_session_release(sess);
    return rc;
}

static void copy_cstr(char *dst, size_t dstcap, NSString *s) {
    if (dstcap == 0) return;
    const char *src = s ? [s UTF8String] : NULL;
    if (!src) src = "";
    strncpy(dst, src, dstcap - 1);
    dst[dstcap - 1] = '\0';
}

// ---- Public C ABI -----------------------------------------------------------

int dvpn_list(dvpn_service_t **services, int *count) {
    if (!services || !count) return DVPN_ERR_INTERNAL;
    *services = NULL;
    *count = 0;

    @autoreleasepool {
        int err = DVPN_OK;
        NSArray *configurations = load_configurations(&err);
        if (!configurations) return err;

        NSMutableArray *filtered = [NSMutableArray array];
        for (id cfg in configurations) {
            if (is_vpn(cfg)) [filtered addObject:cfg];
        }
        if (filtered.count == 0) return DVPN_OK;

        dvpn_service_t *arr = calloc(filtered.count, sizeof(dvpn_service_t));
        if (!arr) return DVPN_ERR_INTERNAL;

        int i = 0;
        for (id cfg in filtered) {
            NSString *name = [cfg performSelector:@selector(name)];
            NSUUID *uid = [cfg performSelector:@selector(identifier)];

            copy_cstr(arr[i].name, sizeof(arr[i].name), name);
            copy_cstr(arr[i].uuid, sizeof(arr[i].uuid),
                      uid ? [uid UUIDString] : nil);

            int status = 0;
            if (uid && fetch_status(uid, &status) != DVPN_OK) {
                status = 0;
            }
            arr[i].status = status;
            i++;
        }

        *services = arr;
        *count = (int)filtered.count;
    }
    return DVPN_OK;
}

// Status integers per ne_session_status_t (verified in docs/phase1-probe).
#define NE_STATUS_INVALID        0
#define NE_STATUS_DISCONNECTED   1
#define NE_STATUS_CONNECTING     2
#define NE_STATUS_CONNECTED      3
#define NE_STATUS_REASSERTING    4
#define NE_STATUS_DISCONNECTING  5

int dvpn_start(const char *uuid) {
    if (!uuid) return DVPN_ERR_INTERNAL;
    @autoreleasepool {
        int err = DVPN_OK;
        id cfg = find_configuration(uuid, &err);
        if (!cfg) return err;
        NSUUID *uid = [cfg performSelector:@selector(identifier)];

        int current = NE_STATUS_INVALID;
        int rc = fetch_status(uid, &current);
        if (rc != DVPN_OK) return rc;
        switch (current) {
            case NE_STATUS_CONNECTING:
            case NE_STATUS_CONNECTED:
            case NE_STATUS_REASSERTING:
                return DVPN_ERR_ALREADY;
            default:
                break;
        }

        uuid_t raw;
        [uid getUUIDBytes:raw];
        ne_session_t sess = ne_session_create(raw, NESESSION_TYPE_VPN);
        if (!sess) return DVPN_ERR_INTERNAL;
        ne_session_start(sess);
        ne_session_release(sess);
    }
    return DVPN_OK;
}

int dvpn_stop(const char *uuid) {
    if (!uuid) return DVPN_ERR_INTERNAL;
    @autoreleasepool {
        int err = DVPN_OK;
        id cfg = find_configuration(uuid, &err);
        if (!cfg) return err;
        NSUUID *uid = [cfg performSelector:@selector(identifier)];

        int current = NE_STATUS_INVALID;
        int rc = fetch_status(uid, &current);
        if (rc != DVPN_OK) return rc;
        if (current == NE_STATUS_INVALID || current == NE_STATUS_DISCONNECTED) {
            return DVPN_ERR_NOT_ACTIVE;
        }

        uuid_t raw;
        [uid getUUIDBytes:raw];
        ne_session_t sess = ne_session_create(raw, NESESSION_TYPE_VPN);
        if (!sess) return DVPN_ERR_INTERNAL;
        ne_session_stop(sess);
        ne_session_release(sess);
    }
    return DVPN_OK;
}

int dvpn_status(const char *uuid, int *status_out) {
    if (!uuid || !status_out) return DVPN_ERR_INTERNAL;
    *status_out = 0;
    @autoreleasepool {
        int err = DVPN_OK;
        id cfg = find_configuration(uuid, &err);
        if (!cfg) return err;
        NSUUID *uid = [cfg performSelector:@selector(identifier)];
        return fetch_status(uid, status_out);
    }
}

// fill_protocol_fields populates server_address / remote_identifier / username
// from the IKEv2 protocol payload hanging off NEConfiguration.VPN.protocol.
// NEVPNConfiguration is private but its `protocol` property has been stable
// since macOS 10.11 (NetworkExtension was introduced) and is what the public
// NEVPNManager.protocolConfiguration accessor returns. Non-IKEv2 protocols
// (NETunnelProviderProtocol) are silently skipped — the fields stay empty.
static void fill_protocol_fields(id cfg, dvpn_status_detail_t *out) {
    if (!cfg || !out) return;
    id vpnCfg = nil;
    @try {
        vpnCfg = [cfg performSelector:@selector(VPN)];
    } @catch (NSException *e) {
        return;
    }
    if (!vpnCfg) return;
    SEL protoSel = NSSelectorFromString(@"protocol");
    if (![vpnCfg respondsToSelector:protoSel]) return;
    id proto = [vpnCfg performSelector:protoSel];
    if (![proto isKindOfClass:[NEVPNProtocolIKEv2 class]]) return;
    NEVPNProtocolIKEv2 *ike = (NEVPNProtocolIKEv2 *)proto;
    copy_cstr(out->server_address, sizeof(out->server_address), ike.serverAddress);
    copy_cstr(out->remote_identifier, sizeof(out->remote_identifier), ike.remoteIdentifier);
    copy_cstr(out->username, sizeof(out->username), ike.username);
}

// copy_xpc_string_into copies the first element of the "Addresses" array of
// the family ("IPv4" / "IPv6") sub-dictionary into dst. Silently no-ops if
// the path is missing or the value isn't a string.
static void copy_first_address(xpc_object_t info, const char *family,
                               char *dst, size_t dstcap) {
    if (!info || !family || !dst || dstcap == 0) return;
    dst[0] = '\0';
    xpc_object_t fam = xpc_dictionary_get_value(info, family);
    if (!fam || xpc_get_type(fam) != XPC_TYPE_DICTIONARY) return;
    xpc_object_t addrs = xpc_dictionary_get_value(fam, "Addresses");
    if (!addrs || xpc_get_type(addrs) != XPC_TYPE_ARRAY) return;
    if (xpc_array_get_count(addrs) == 0) return;
    xpc_object_t addr = xpc_array_get_value(addrs, 0);
    if (!addr || xpc_get_type(addr) != XPC_TYPE_STRING) return;
    const char *s = xpc_string_get_string_ptr(addr);
    if (!s) return;
    strncpy(dst, s, dstcap - 1);
    dst[dstcap - 1] = '\0';
}

// fetch_session_state reads ConnectedAt and the bound IPv4/IPv6 addresses
// from the kind=2 ne_session_get_info dictionary in a single round trip.
// Only meaningful for sessions currently in NE_STATUS_CONNECTED; for other
// states LastStatusChangeTime reflects a different transition and the
// address arrays would typically be absent.
//
// All writes go through __block scratch buffers and are committed to *out
// only when the semaphore wait succeeds, so a timeout cannot leave the
// caller with a torn payload.
static void fetch_session_state(NSUUID *uid, dvpn_status_detail_t *out) {
    if (!uid || !out) return;
    uuid_t raw;
    [uid getUUIDBytes:raw];
    ne_session_t sess = ne_session_create(raw, NESESSION_TYPE_VPN);
    if (!sess) return;

    // Blocks cannot capture array-typed variables, so wrap the scratch
    // buffers in a struct.
    struct session_scratch {
        int64_t connected_at_unix;
        char    ipv4[sizeof(out->ipv4_address)];
        char    ipv6[sizeof(out->ipv6_address)];
    };
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block struct session_scratch observed = {0};

    ne_session_get_info(sess, NE_SESSION_INFO_KIND_STATE, bridge_queue(),
                        ^(xpc_object_t info) {
        if (info && xpc_get_type(info) == XPC_TYPE_DICTIONARY) {
            xpc_object_t d = xpc_dictionary_get_value(info,
                                                      "LastStatusChangeTime");
            if (d && xpc_get_type(d) == XPC_TYPE_DATE) {
                // xpc_date_get_value returns nanoseconds since the UNIX
                // epoch (verified by probe3.m output matching wall time).
                int64_t nanos = xpc_date_get_value(d);
                observed.connected_at_unix = nanos / (int64_t)NSEC_PER_SEC;
            }
            copy_first_address(info, "IPv4", observed.ipv4, sizeof(observed.ipv4));
            copy_first_address(info, "IPv6", observed.ipv6, sizeof(observed.ipv6));
        }
        dispatch_semaphore_signal(sem);
    });

    if (dispatch_semaphore_wait(sem,
                                dispatch_time(DISPATCH_TIME_NOW,
                                              5 * NSEC_PER_SEC)) == 0) {
        out->connected_at_unix = observed.connected_at_unix;
        memcpy(out->ipv4_address, observed.ipv4, sizeof(observed.ipv4));
        memcpy(out->ipv6_address, observed.ipv6, sizeof(observed.ipv6));
    }
    ne_session_release(sess);
}

int dvpn_status_detail(const char *uuid, dvpn_status_detail_t *out) {
    if (!uuid || !out) return DVPN_ERR_INTERNAL;
    memset(out, 0, sizeof(*out));
    @autoreleasepool {
        int err = DVPN_OK;
        id cfg = find_configuration(uuid, &err);
        if (!cfg) return err;
        NSUUID *uid = [cfg performSelector:@selector(identifier)];
        int rc = fetch_status(uid, &out->status);
        if (rc != DVPN_OK) return rc;
        fill_protocol_fields(cfg, out);
        if (out->status == NE_STATUS_CONNECTED) {
            fetch_session_state(uid, out);
        }
    }
    return DVPN_OK;
}
