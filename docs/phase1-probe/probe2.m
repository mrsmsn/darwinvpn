// probe2.m — extended probe for the scope-widening decision.
// Goal: for every NEConfiguration that has a non-nil VPN payload, dump the
// VPN.protocol class name and observe ne_session_get_status when we create
// a session as NESessionTypeVPN=1.
//
// Read-only: no Start/Stop is issued; nothing actually changes on the host.
//
// Build:
//   clang -fobjc-arc -framework Foundation -framework NetworkExtension \
//       probe2.m -o probe2
// Run:
//   ./probe2

#import <Foundation/Foundation.h>
#import <dispatch/dispatch.h>
#import <objc/runtime.h>

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

// Known ne_session_create sessionType integers (Timac / WebKit / RE):
//   1 = NESessionTypeVPN          (Personal VPN: IKEv2, IPSec, L2TP)
//   2 = NESessionTypeAppVPN
//   3 = NESessionTypeContentFilter
//   4 = NESessionTypeAlwaysOnVPN
//   5 = NESessionTypePacketTunnel  (NetworkExtension Tunnel Provider, e.g. Tailscale, WireGuard, OpenVPN)
//   6 = NESessionTypeFlowDivert
//   7 = NESessionTypePathController
//   8 = NESessionTypeDNSProxy
//   9 = NESessionTypePluginVPN
static const int kSessionTypeVPN          = 1;
static const int kSessionTypePacketTunnel = 5;
static const int kSessionTypePluginVPN    = 9;

static int probe_session(NSUUID *uid, int sessionType, dispatch_queue_t q) {
    uuid_t raw;
    [uid getUUIDBytes:raw];
    ne_session_t sess = ne_session_create(raw, sessionType);
    if (!sess) {
        printf("      [type=%d] ne_session_create returned NULL\n", sessionType);
        return -1;
    }
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block int observed = -1;
    ne_session_get_status(sess, q, ^(int s) {
        observed = s;
        dispatch_semaphore_signal(sem);
    });
    int rc;
    if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW,
                                                   3 * NSEC_PER_SEC)) != 0) {
        printf("      [type=%d] ne_session_get_status TIMED OUT\n", sessionType);
        rc = -2;
    } else {
        printf("      [type=%d] status -> %d (%s)\n",
               sessionType, observed, status_name(observed));
        rc = observed;
    }
    ne_session_release(sess);
    return rc;
}

int main(int argc, char **argv) {
    @autoreleasepool {
        printf("=== probe2: extended VPN session-type probe ===\n");
        printf("macOS version: ");
        fflush(stdout);
        system("sw_vers -productVersion");

        Class mgrClass = NSClassFromString(@"NEConfigurationManager");
        if (!mgrClass) { printf("FAIL: NEConfigurationManager missing\n"); return 1; }
        id mgr = [mgrClass performSelector:@selector(sharedManager)];

        dispatch_queue_t q = dispatch_queue_create("probe2.q", DISPATCH_QUEUE_SERIAL);
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block NSArray *configs = nil;
        SEL loadSel = NSSelectorFromString(
            @"loadConfigurationsWithCompletionQueue:handler:");
        void (*loadFn)(id, SEL, dispatch_queue_t, id) =
            (void (*)(id, SEL, dispatch_queue_t, id))
                [mgr methodForSelector:loadSel];
        loadFn(mgr, loadSel, q, ^(NSArray *r, NSError *err) {
            configs = r;
            (void)err;
            dispatch_semaphore_signal(sem);
        });
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW,
                                                       10 * NSEC_PER_SEC)) != 0) {
            printf("FAIL: load timed out\n"); return 1;
        }
        if (!configs) { printf("FAIL: load returned nil\n"); return 1; }

        printf("Loaded %lu configuration(s).\n\n", (unsigned long)configs.count);
        for (id cfg in configs) {
            NSString *name = [cfg performSelector:@selector(name)];
            NSUUID *uid = [cfg performSelector:@selector(identifier)];
            id vpn = nil;
            @try { vpn = [cfg performSelector:@selector(VPN)]; } @catch(...) {}

            printf("- %s [%s]\n",
                   [name UTF8String] ?: "(nil)",
                   [[uid UUIDString] UTF8String] ?: "(nil)");
            if (!vpn) {
                printf("    VPN = nil  (skip; not a VPN-type configuration)\n\n");
                continue;
            }
            printf("    VPN class      = %s\n",
                   class_getName([vpn class]) ?: "(unknown)");

            SEL protoSel = NSSelectorFromString(@"protocol");
            id protocol = nil;
            if ([vpn respondsToSelector:protoSel]) {
                protocol = ((id (*)(id, SEL))[vpn methodForSelector:protoSel])(vpn, protoSel);
            }
            printf("    protocol class = %s\n",
                   protocol ? (class_getName([protocol class]) ?: "(unknown)") : "(nil)");

            // Try the three plausible sessionType values for VPN-like configs.
            probe_session(uid, kSessionTypeVPN,          q);
            probe_session(uid, kSessionTypePacketTunnel, q);
            probe_session(uid, kSessionTypePluginVPN,    q);
            printf("\n");
        }
        printf("=== reference sessionType integers ===\n");
        printf("  1 = NESessionTypeVPN           (IKEv2 / IPSec / L2TP)\n");
        printf("  5 = NESessionTypePacketTunnel  (Tunnel Provider: Tailscale, WG, OpenVPN)\n");
        printf("  9 = NESessionTypePluginVPN     (legacy plugin-based VPN)\n");
    }
    return 0;
}
