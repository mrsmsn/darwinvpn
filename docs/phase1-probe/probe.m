// Phase 1 §6.5 probe — verify the private NetworkExtension API surface that
// the darwinvpn cgo bridge will depend on. Run this on the target macOS
// version(s) and paste the output back so the integer constants used by the
// Go side can be locked down.
//
// Build (link-mode A, preferred):
//   clang -fobjc-arc -framework Foundation -framework NetworkExtension \
//       probe.m -o probe
// Run:
//   ./probe
//
// If clang reports unresolved symbols for ne_session_*, fall back to the
// dlopen approach described in docs/phase1-probe/README.md.

#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>
#import <stdio.h>

// ---- Private declarations (Timac/VPNStatus, blog.timac.org) ----
typedef struct ne_session_s *ne_session_t;

extern ne_session_t ne_session_create(uuid_t serviceID, int sessionConfigType);
extern void ne_session_release(ne_session_t session);

typedef void (^ne_session_status_block)(int status);
extern void ne_session_get_status(ne_session_t session,
                                  dispatch_queue_t queue,
                                  ne_session_status_block block);

// NEConfigurationManager is private; declared here so the compiler resolves
// the selectors via NSObject's dynamic dispatch.
@interface NEConfiguration : NSObject
@property (readonly) NSUUID *identifier;
@property (readonly) NSString *name;
@end

@interface NEConfigurationManager : NSObject
+ (instancetype)sharedManager;
- (void)loadConfigurationsWithCompletionQueue:(dispatch_queue_t)queue
                                      handler:(void (^)(NSArray<NEConfiguration *> *, NSError *))handler;
@end

static const char *status_name(int s) {
    switch (s) {
        case 0: return "Invalid";
        case 1: return "Disconnected";
        case 2: return "Connecting";
        case 3: return "Connected";
        case 4: return "Reasserting";
        case 5: return "Disconnecting";
        default: return "Unknown";
    }
}

int main(int argc, char **argv) {
    @autoreleasepool {
        printf("=== probe: NetworkExtension private API check ===\n");
        printf("macOS version: ");
        fflush(stdout);
        system("sw_vers -productVersion");
        printf("architecture:  ");
        fflush(stdout);
        system("uname -m");

        // --- 1) NEConfigurationManager class lookup ---
        Class mgrClass = NSClassFromString(@"NEConfigurationManager");
        if (!mgrClass) {
            printf("FAIL: NEConfigurationManager class not found in runtime\n");
            return 1;
        }
        printf("OK: NEConfigurationManager class resolved (%p)\n", (__bridge void *)mgrClass);

        id mgr = [mgrClass performSelector:@selector(sharedManager)];
        if (!mgr) {
            printf("FAIL: +[NEConfigurationManager sharedManager] returned nil\n");
            return 1;
        }
        printf("OK: +sharedManager returned an instance\n");

        // --- 2) Load configurations (async, sync via semaphore) ---
        SEL loadSel = NSSelectorFromString(@"loadConfigurationsWithCompletionQueue:handler:");
        if (![mgr respondsToSelector:loadSel]) {
            printf("FAIL: loadConfigurationsWithCompletionQueue:handler: selector unavailable\n");
            return 1;
        }

        dispatch_queue_t q = dispatch_queue_create("probe.q", DISPATCH_QUEUE_SERIAL);
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block NSArray *configurations = nil;
        __block NSError *loadError = nil;

        void (*loadFn)(id, SEL, dispatch_queue_t, id) =
            (void (*)(id, SEL, dispatch_queue_t, id))[mgr methodForSelector:loadSel];
        loadFn(mgr, loadSel, q, ^(NSArray *configs, NSError *err) {
            configurations = configs;
            loadError = err;
            dispatch_semaphore_signal(sem);
        });
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW,
                                                       10 * NSEC_PER_SEC)) != 0) {
            printf("FAIL: load timed out after 10s\n");
            return 1;
        }
        if (loadError) {
            printf("FAIL: load error: %s\n",
                   [[loadError description] UTF8String]);
            return 1;
        }
        printf("OK: loaded %lu configuration(s)\n",
               (unsigned long)configurations.count);
        for (id cfg in configurations) {
            NSUUID *uid = [cfg performSelector:@selector(identifier)];
            NSString *name = [cfg performSelector:@selector(name)];
            printf("  - %-30s [%s]\n",
                   [name UTF8String] ?: "(nil)",
                   [[uid UUIDString] UTF8String] ?: "(nil)");
        }

        if (configurations.count == 0) {
            printf("\nINFO: no VPN configuration registered.\n");
            printf("      Create one in System Settings > Network and rerun.\n");
            printf("      (ne_session_* probing is skipped.)\n");
            return 0;
        }

        // --- 3) ne_session_* link + status read ---
        printf("\n=== ne_session_* link check ===\n");
        printf("ne_session_create     address: %p\n",
               (void *)&ne_session_create);
        printf("ne_session_get_status address: %p\n",
               (void *)&ne_session_get_status);
        printf("ne_session_release    address: %p\n",
               (void *)&ne_session_release);

        id firstCfg = configurations[0];
        NSUUID *uid = [firstCfg performSelector:@selector(identifier)];
        NSString *name = [firstCfg performSelector:@selector(name)];
        uuid_t raw;
        [uid getUUIDBytes:raw];

        // Timac's reverse-engineering reports NESessionTypeVPN == 1.
        const int kNESessionTypeVPN = 1;
        ne_session_t sess = ne_session_create(raw, kNESessionTypeVPN);
        if (!sess) {
            printf("FAIL: ne_session_create returned NULL (NESessionTypeVPN=%d)\n",
                   kNESessionTypeVPN);
            return 1;
        }
        printf("OK: ne_session_create succeeded for %s\n",
               [name UTF8String]);

        dispatch_semaphore_t sem2 = dispatch_semaphore_create(0);
        __block int observedStatus = -1;
        ne_session_get_status(sess, q, ^(int status) {
            observedStatus = status;
            dispatch_semaphore_signal(sem2);
        });
        if (dispatch_semaphore_wait(sem2,
                                    dispatch_time(DISPATCH_TIME_NOW,
                                                  5 * NSEC_PER_SEC)) != 0) {
            printf("FAIL: ne_session_get_status timed out\n");
            ne_session_release(sess);
            return 1;
        }
        printf("OK: ne_session_get_status -> %d (%s)\n",
               observedStatus, status_name(observedStatus));
        ne_session_release(sess);

        printf("\n=== reference enum values (Timac/VPNStatus) ===\n");
        printf("  NESessionTypeVPN                = 1\n");
        printf("  NESessionStatusInvalid          = 0\n");
        printf("  NESessionStatusDisconnected     = 1\n");
        printf("  NESessionStatusConnecting       = 2\n");
        printf("  NESessionStatusConnected        = 3\n");
        printf("  NESessionStatusReasserting      = 4\n");
        printf("  NESessionStatusDisconnecting    = 5\n");
        printf("If the observed value matches one of the names above for the\n");
        printf("VPN's actual current state, the enum mapping is confirmed.\n");
    }
    return 0;
}
