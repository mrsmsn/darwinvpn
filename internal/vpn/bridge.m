// bridge.m — Objective-C implementation of the dvpn_* C ABI declared in
// bridge.h. See docs/pj.md §6 for the architectural background and
// docs/phase1-probe/ for the runtime verification that motivates the IKEv2
// filter and the enum mapping used below.

#include "bridge.h"

#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>
#import <string.h>
#import <stdlib.h>

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

// is_ikev2 returns YES when configuration represents an IKEv2 Personal VPN.
// The probe (docs/phase1-probe) showed NEConfigurationManager returns
// firewall, Network Privacy, and Tunnel Provider entries as well; we must
// filter to IKEv2 only.
static BOOL is_ikev2(id configuration) {
    if (!configuration) return NO;
    id vpn = nil;
    @try {
        vpn = [configuration performSelector:@selector(VPN)];
    } @catch (NSException *e) {
        return NO;
    }
    if (!vpn) return NO;

    SEL protoSel = NSSelectorFromString(@"protocol");
    if (![vpn respondsToSelector:protoSel]) return NO;
    id protocol = ((id (*)(id, SEL))[vpn methodForSelector:protoSel])(vpn, protoSel);
    if (!protocol) return NO;

    Class ikev2Class = NSClassFromString(@"NEVPNProtocolIKEv2");
    if (!ikev2Class) return NO;
    return [protocol isKindOfClass:ikev2Class];
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
            if (is_ikev2(cfg)) [filtered addObject:cfg];
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
