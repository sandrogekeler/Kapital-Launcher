// An OS activity for the game tracker (activity_darwin.go). Compiled with ARC
// (activity_darwin.go's CFLAGS), like backend/splashhost/host_darwin.m.
//
// While a game runs the launcher is minimised and the loading card is closed,
// so the process qualifies for App Nap, which coalesces timers: the tracker's
// one second wait on the game and its log follower could lag by seconds. An
// activity opts the process out until it is ended.
// https://developer.apple.com/library/archive/documentation/Performance/Conceptual/power_efficiency_guidelines_osx/AppNap.html

#import <Foundation/Foundation.h>

#include "activity_darwin.h"

// These run on Go's threads, which have no autorelease pool of their own.

void *kapitalBeginActivity(const char *reason) {
    @autoreleasepool {
        NSString *why = reason ? [NSString stringWithUTF8String:reason] : nil;
        if (why == nil) why = @"following a game";
        // UserInitiatedAllowingIdleSystemSleep is user initiated work that does
        // not keep the machine awake when it is idle: this is about timers, not
        // about holding the Mac up.
        // https://developer.apple.com/documentation/foundation/processinfo/beginactivity(options:reason:)
        id<NSObject> token = [[NSProcessInfo processInfo]
            beginActivityWithOptions:NSActivityUserInitiatedAllowingIdleSystemSleep
                              reason:why];
        // Retained for Go to hold: the token is released in kapitalEndActivity.
        return (__bridge_retained void *)token;
    }
}

void kapitalEndActivity(void *ref) {
    if (ref == NULL) return;
    @autoreleasepool {
        id<NSObject> token = (__bridge_transfer id<NSObject>)ref;
        [[NSProcessInfo processInfo] endActivity:token];
    }
}
