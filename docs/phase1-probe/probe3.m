// probe3.m — probe for the connectedDate / session start time retrieval path.
// Goal: discover how to obtain the timestamp at which the current VPN session
// entered Connected so darwinvpn status can render an elapsed "Connected: ..."
// line. NetworkExtension does not expose a public API for this on the
// NEConfiguration path used by bridge.m, so we runtime-introspect to find one.
//
// Strategy:
//   1. Dump every Objective-C property on NEConfiguration and the object
//      returned by NEConfiguration.VPN (private NEVPNConfiguration). Look
//      especially for `connection`, `connectedDate`, or anything date-shaped.
//   2. If a `connection` property exists, dump its class and properties too
//      (it is presumably NEVPNConnection, whose public docs list
//      `connectedDate`).
//   3. Probe the libsystem_networkextension.dylib symbol table for
//      ne_session_get_info; if present, call it with a few plausible `kind`
//      values to see what dictionaries come back.
//
// Read-only: no Start/Stop is issued; nothing changes on the host.
//
// Build:
//   clang -fobjc-arc -framework Foundation -framework NetworkExtension \
//       probe3.m -o probe3
// Run:
//   ./probe3

#import <Foundation/Foundation.h>
#import <NetworkExtension/NetworkExtension.h>
#import <dispatch/dispatch.h>
#import <objc/runtime.h>
#import <dlfcn.h>

typedef struct ne_session_s *ne_session_t;
extern ne_session_t ne_session_create(uuid_t serviceID, int sessionConfigType);
extern void ne_session_release(ne_session_t session);
typedef void (^ne_session_status_block)(int status);
extern void ne_session_get_status(ne_session_t session,
                                  dispatch_queue_t queue,
                                  ne_session_status_block block);

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

static void dump_properties(const char *label, id obj) {
    if (!obj) {
        printf("    %s = nil\n", label);
        return;
    }
    Class cls = [obj class];
    printf("    %s class = %s\n", label, class_getName(cls));
    unsigned count = 0;
    objc_property_t *props = class_copyPropertyList(cls, &count);
    for (unsigned i = 0; i < count; i++) {
        const char *name = property_getName(props[i]);
        const char *attr = property_getAttributes(props[i]);
        printf("      .%s  %s\n", name ?: "(null)", attr ?: "");
    }
    if (props) free(props);
}

// Try invoking a selector that takes no arguments and returns an id. Used to
// pull values out of private properties whose declaration we don't have.
static id call_id_selector(id receiver, SEL sel) {
    if (!receiver || !sel) return nil;
    if (![receiver respondsToSelector:sel]) return nil;
    IMP imp = [receiver methodForSelector:sel];
    id (*fn)(id, SEL) = (id (*)(id, SEL))imp;
    @try { return fn(receiver, sel); }
    @catch (NSException *e) { return nil; }
}

static void probe_connection_object(id vpnCfg) {
    SEL sels[] = {
        @selector(connection),
        NSSelectorFromString(@"VPNConnection"),
        NSSelectorFromString(@"session"),
    };
    for (size_t i = 0; i < sizeof(sels) / sizeof(sels[0]); i++) {
        SEL s = sels[i];
        if (![vpnCfg respondsToSelector:s]) continue;
        id conn = call_id_selector(vpnCfg, s);
        printf("    -[VPN %s] = %p", sel_getName(s), (__bridge void *)conn);
        if (!conn) { printf("\n"); continue; }
        printf("  (%s)\n", class_getName([conn class]));
        dump_properties("      connection", conn);

        SEL cd = NSSelectorFromString(@"connectedDate");
        if ([conn respondsToSelector:cd]) {
            id date = call_id_selector(conn, cd);
            printf("      -[connection connectedDate] = %s\n",
                   date ? [[date description] UTF8String] : "(nil)");
        } else {
            printf("      (no -connectedDate selector)\n");
        }
    }
}

static void probe_ne_session_get_info(void) {
    printf("\n=== libsystem_networkextension.dylib symbol probe ===\n");
    void *h = dlopen("/usr/lib/system/libsystem_networkextension.dylib",
                     RTLD_LAZY | RTLD_GLOBAL);
    if (!h) {
        printf("  dlopen failed: %s\n", dlerror());
        return;
    }
    const char *names[] = {
        "ne_session_get_info",
        "ne_session_copy_info",
        "ne_session_get_connected_date",
        "ne_session_copy_connected_date",
        "ne_session_get_statistics",
    };
    for (size_t i = 0; i < sizeof(names) / sizeof(names[0]); i++) {
        void *p = dlsym(h, names[i]);
        printf("  %s = %p\n", names[i], p);
    }

    // If ne_session_get_info exists, it most plausibly has the signature
    //   void ne_session_get_info(ne_session_t, int kind, dispatch_queue_t,
    //                            void (^)(NSObject *info));
    // Try kinds 1..6 against an arbitrary session to see what comes back.
    typedef void (^info_block)(id info);
    typedef void (*ne_session_get_info_fn)(ne_session_t, int,
                                           dispatch_queue_t, info_block);
    ne_session_get_info_fn fn =
        (ne_session_get_info_fn)dlsym(h, "ne_session_get_info");
    if (!fn) { dlclose(h); return; }

    printf("\n  ne_session_get_info appears callable; will try with each\n"
           "  loaded VPN session at the bottom of the run.\n");
    // Don't close h: keep symbols valid for callers below.
    (void)fn;
}

static void try_get_info_for_uuid(NSUUID *uid, NSString *name) {
    void *h = dlopen("/usr/lib/system/libsystem_networkextension.dylib",
                     RTLD_LAZY | RTLD_GLOBAL);
    if (!h) return;
    typedef void (^info_block)(id info);
    typedef void (*ne_session_get_info_fn)(ne_session_t, int,
                                           dispatch_queue_t, info_block);
    ne_session_get_info_fn fn =
        (ne_session_get_info_fn)dlsym(h, "ne_session_get_info");
    if (!fn) { dlclose(h); return; }

    uuid_t raw;
    [uid getUUIDBytes:raw];
    ne_session_t sess = ne_session_create(raw, 1 /* NESessionTypeVPN */);
    if (!sess) { printf("    ne_session_create returned NULL\n"); return; }

    dispatch_queue_t q = dispatch_queue_create("probe3.info", DISPATCH_QUEUE_SERIAL);
    for (int kind = 1; kind <= 6; kind++) {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block id captured = nil;
        @try {
            fn(sess, kind, q, ^(id info) {
                captured = info;
                dispatch_semaphore_signal(sem);
            });
        } @catch (NSException *e) {
            printf("    [kind=%d] threw: %s\n", kind, [[e description] UTF8String]);
            continue;
        }
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW,
                                                       2 * NSEC_PER_SEC)) != 0) {
            printf("    [kind=%d] TIMED OUT\n", kind);
            continue;
        }
        if (!captured) {
            printf("    [kind=%d] (nil info)\n", kind);
            continue;
        }
        printf("    [kind=%d] %s = %s\n", kind,
               class_getName([captured class]),
               [[captured description] UTF8String]);
    }
    ne_session_release(sess);
    (void)name;
}

int main(int argc, char **argv) {
    @autoreleasepool {
        printf("=== probe3: connectedDate path probe ===\n");
        printf("macOS version: "); fflush(stdout); system("sw_vers -productVersion");

        Class mgrClass = NSClassFromString(@"NEConfigurationManager");
        if (!mgrClass) { printf("FAIL: NEConfigurationManager missing\n"); return 1; }
        id mgr = [mgrClass performSelector:@selector(sharedManager)];

        dispatch_queue_t q = dispatch_queue_create("probe3.q", DISPATCH_QUEUE_SERIAL);
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block NSArray *configs = nil;
        SEL loadSel = NSSelectorFromString(
            @"loadConfigurationsWithCompletionQueue:handler:");
        void (*loadFn)(id, SEL, dispatch_queue_t, id) =
            (void (*)(id, SEL, dispatch_queue_t, id))
                [mgr methodForSelector:loadSel];
        loadFn(mgr, loadSel, q, ^(NSArray *r, NSError *err) {
            configs = r; (void)err; dispatch_semaphore_signal(sem);
        });
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW,
                                                       10 * NSEC_PER_SEC)) != 0) {
            printf("FAIL: load timed out\n"); return 1;
        }
        if (!configs) { printf("FAIL: load returned nil\n"); return 1; }

        printf("Loaded %lu configuration(s).\n\n", (unsigned long)configs.count);

        BOOL dumpedConfigClass = NO;
        for (id cfg in configs) {
            NSString *name = [cfg performSelector:@selector(name)];
            NSUUID *uid = [cfg performSelector:@selector(identifier)];
            id vpn = [cfg performSelector:@selector(VPN)];
            if (!vpn) continue;

            printf("=== %s [%s] ===\n",
                   [name UTF8String] ?: "(nil)",
                   [[uid UUIDString] UTF8String] ?: "(nil)");

            if (!dumpedConfigClass) {
                dump_properties("NEConfiguration", cfg);
                dumpedConfigClass = YES;
            }
            dump_properties("VPN payload", vpn);
            probe_connection_object(vpn);
            try_get_info_for_uuid(uid, name);
            printf("\n");
        }
        probe_ne_session_get_info();
    }
    return 0;
}
