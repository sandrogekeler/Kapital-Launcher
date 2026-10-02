// The loading card's window on macOS (#97): a borderless NSWindow holding a
// WKWebView of its own, in the shape of Wails' WailsContext.m (a scheme handler
// answering from memory, a script message handler, calls dispatched to the main
// queue). Wails owns NSApp and its run loop; nothing here runs one. Compiled
// with ARC (host_darwin.go's CFLAGS), unlike Wails' own files.
//
// Go and this file cross by handle, never by Go pointer: each callback below
// carries the integer host_darwin.go registered the card under, and Go answers
// "no such card" once it has been closed.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <ApplicationServices/ApplicationServices.h>

#include "host_darwin.h"

// Implemented in host_darwin.go (//export). Spelled here by hand: see the
// header for why.
extern void splashNavigated(uintptr_t handle, int ok, const char *err);
extern int splashAsset(uintptr_t handle, const char *url, char **body, size_t *bodyLen, char **mime);
extern void splashMessage(uintptr_t handle, const char *msg);

// The name the page posts through: window.webkit.messageHandlers.splash
// (protocol.go).
static NSString *const kMessageHandlerName = @"splash";

// A page's message is a few dozen bytes; anything past this is not ours.
static const NSUInteger kMaxMessageLength = 4096;

// How long the state is retried while the page has not defined
// window.kapitalSplash yet: 50 tries, 100 ms apart.
static const int kMaxEvalRetries = 50;

// A borderless window refuses to be key unless it says it can; the card needs
// to be, for its buttons to take the first click. It is never main, so the
// launcher stays the app's main window.
@interface KSplashWindow : NSWindow
@end

@implementation KSplashWindow
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return NO; }
@end

// One card: the window, its webview and the three delegate roles. Every method
// runs on the main queue.
@interface KSplash : NSObject <WKURLSchemeHandler, WKScriptMessageHandler, WKNavigationDelegate>
@property (nonatomic) uintptr_t handle;
@property (nonatomic, copy) NSString *scheme;
@property (nonatomic, strong) KSplashWindow *window;
@property (nonatomic, strong) WKWebView *webView;
// The newest state not yet delivered, and whether the page has loaded to take it.
@property (nonatomic, copy) NSString *pending;
@property (nonatomic) BOOL loaded;
@property (nonatomic) BOOL reported;
@property (nonatomic) BOOL closed;
@property (nonatomic) int retries;
@end

@implementation KSplash

- (void)report:(BOOL)ok message:(NSString *)message {
    if (self.reported) return;
    self.reported = YES;
    splashNavigated(self.handle, ok ? 1 : 0, message.UTF8String);
}

// flush delivers the newest state if the page has loaded. The expression is
// protocol.go's, written as a conditional so a page that has not defined
// window.kapitalSplash yet answers false and the state is tried again.
- (void)flush {
    if (self.closed || !self.loaded || self.pending == nil) return;
    NSString *state = self.pending;
    NSString *script = [NSString stringWithFormat:
        @"window.kapitalSplash ? (window.kapitalSplash.update(%@), true) : false", state];
    __weak KSplash *weakSelf = self;
    [self.webView evaluateJavaScript:script completionHandler:^(id result, NSError *error) {
        KSplash *strongSelf = weakSelf;
        if (strongSelf == nil || strongSelf.closed) return;
        if ([result isKindOfClass:[NSNumber class]] && [result boolValue]) {
            if ([strongSelf.pending isEqualToString:state]) strongSelf.pending = nil;
            return;
        }
        if (strongSelf.retries++ >= kMaxEvalRetries) return;
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 100 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
            [weakSelf flush];
        });
    }];
}

// teardown takes the card apart so nothing it owns keeps anything else alive:
// the user content controller retains its script message handler, which is this
// object.
- (void)teardown {
    self.closed = YES;
    self.pending = nil;
    [self.webView stopLoading];
    self.webView.navigationDelegate = nil;
    [self.webView.configuration.userContentController removeScriptMessageHandlerForName:kMessageHandlerName];
    [self.window orderOut:nil];
    [self.webView removeFromSuperview];
    [self.window close];
    self.webView = nil;
    self.window = nil;
}

#pragma mark WKURLSchemeHandler

// Every request is answered at once, on the main queue, from the Go side's
// Page.Assets: a body and a type, or a 404. Nothing is fetched from anywhere.
- (void)webView:(WKWebView *)webView startURLSchemeTask:(id<WKURLSchemeTask>)task {
    NSURL *url = task.request.URL;
    char *body = NULL;
    size_t bodyLen = 0;
    char *mime = NULL;
    int found = splashAsset(self.handle, url.absoluteString.UTF8String, &body, &bodyLen, &mime);

    NSData *data;
    NSString *type;
    if (found) {
        data = [NSData dataWithBytes:body length:bodyLen];
        type = mime ? [NSString stringWithUTF8String:mime] : nil;
    }
    free(body);
    free(mime);
    if (!found) {
        data = [@"not found" dataUsingEncoding:NSUTF8StringEncoding];
        type = @"text/plain; charset=utf-8";
    }
    if (type == nil) type = @"application/octet-stream";

    NSDictionary *headers = @{
        @"Content-Type": type,
        @"Content-Length": [NSString stringWithFormat:@"%lu", (unsigned long)data.length],
        // The page and the build are the same origin; this only spares a font
        // or module request WebKit counts as cross-origin for a custom scheme.
        @"Access-Control-Allow-Origin": @"*",
    };
    NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:url
                                                              statusCode:(found ? 200 : 404)
                                                             HTTPVersion:@"HTTP/1.1"
                                                            headerFields:headers];
    [task didReceiveResponse:response];
    [task didReceiveData:data];
    [task didFinish];
}

// Nothing is ever left pending: each task above is finished before it returns.
- (void)webView:(WKWebView *)webView stopURLSchemeTask:(id<WKURLSchemeTask>)task {
}

#pragma mark WKScriptMessageHandler

- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    if (![message.name isEqualToString:kMessageHandlerName]) return;
    if (!message.frameInfo.mainFrame) return;
    if (![message.frameInfo.request.URL.scheme isEqualToString:self.scheme]) return;
    if (![message.body isKindOfClass:[NSString class]]) return;
    NSString *body = message.body;
    if (body.length > kMaxMessageLength) return;
    splashMessage(self.handle, body.UTF8String);
}

#pragma mark WKNavigationDelegate

// The card never leaves its own scheme: a link, a redirect or a script that
// tries to is cancelled.
- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)action
    decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    BOOL ours = [action.request.URL.scheme isEqualToString:self.scheme];
    decisionHandler(ours ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    self.loaded = YES;
    [self report:YES message:nil];
    [self flush];
}

- (void)webView:(WKWebView *)webView didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    [self report:NO message:error.localizedDescription];
}

- (void)webView:(WKWebView *)webView didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    [self report:NO message:error.localizedDescription];
}

// The page's process dying before the first load ends it would otherwise leave
// Open waiting out its timeout.
- (void)webViewWebContentProcessDidTerminate:(WKWebView *)webView {
    [self report:NO message:@"the page's process ended"];
}

@end

// runOnMain runs block on the main queue and waits up to timeout seconds for it,
// and returns whether it ran. Called on the main thread it just runs, since
// waiting on the queue it is serving would never end. A block the main queue has
// not started by the deadline is cancelled and never runs; one that has started
// is waited for, so a block is never half done when this returns.
static BOOL runOnMain(double timeout, void (^block)(void)) {
    if ([NSThread isMainThread]) {
        block();
        return YES;
    }
    dispatch_semaphore_t finished = dispatch_semaphore_create(0);
    NSLock *lock = [[NSLock alloc] init];
    __block BOOL started = NO;
    __block BOOL cancelled = NO;
    dispatch_async(dispatch_get_main_queue(), ^{
        [lock lock];
        BOOL skip = cancelled;
        if (!skip) started = YES;
        [lock unlock];
        if (!skip) block();
        dispatch_semaphore_signal(finished);
    });
    dispatch_time_t deadline = dispatch_time(DISPATCH_TIME_NOW, (int64_t)(timeout * NSEC_PER_SEC));
    if (dispatch_semaphore_wait(finished, deadline) == 0) return YES;
    [lock lock];
    BOOL ran = started;
    if (!started) cancelled = YES;
    [lock unlock];
    if (!ran) return NO;
    dispatch_semaphore_wait(finished, DISPATCH_TIME_FOREVER);
    return YES;
}

int splashHasWindowServer(void) {
    CFDictionaryRef session = CGSessionCopyCurrentDictionary();
    if (session == NULL) return 0;
    CFRelease(session);
    return 1;
}

// launcherScreen is the screen the launcher's window is on: the app's main or
// key window, else any other window of the app that has one, else the main
// screen.
static NSScreen *launcherScreen(void) {
    NSMutableArray<NSWindow *> *candidates = [NSMutableArray array];
    if (NSApp.mainWindow) [candidates addObject:NSApp.mainWindow];
    if (NSApp.keyWindow) [candidates addObject:NSApp.keyWindow];
    [candidates addObjectsFromArray:NSApp.windows];
    for (NSWindow *window in candidates) {
        if ([window isKindOfClass:[KSplashWindow class]]) continue;
        if (window.screen) return window.screen;
    }
    return NSScreen.mainScreen ?: NSScreen.screens.firstObject;
}

void splashScreenFrame(double *out) {
    out[0] = out[1] = out[2] = out[3] = 0;
    // The screen is read inside the block and only copied out here, so a block
    // that is cancelled leaves the zeros.
    __block NSRect visible = NSZeroRect;
    BOOL ran = runOnMain(5.0, ^{
        visible = launcherScreen().visibleFrame;
    });
    if (!ran) return;
    out[0] = visible.origin.x;
    out[1] = visible.origin.y;
    out[2] = visible.size.width;
    out[3] = visible.size.height;
}

// makeSplash builds the window and webview, or returns nil. Main queue only.
static KSplash *makeSplash(uintptr_t handle, NSRect frame, NSColor *colour, NSString *scheme, NSURL *url) {
    KSplash *splash = [[KSplash alloc] init];
    splash.handle = handle;
    splash.scheme = scheme;

    KSplashWindow *window = [[KSplashWindow alloc] initWithContentRect:frame
                                                             styleMask:NSWindowStyleMaskBorderless
                                                               backing:NSBackingStoreBuffered
                                                                 defer:NO];
    [window setReleasedWhenClosed:NO];
    [window setLevel:NSNormalWindowLevel];
    [window setOpaque:YES];
    [window setHasShadow:YES];
    [window setBackgroundColor:colour];
    // A launcher in native full screen has a Space of its own, and a plain
    // window would open on another one. FullScreenAuxiliary lets the card
    // display on the same Space as a full screen window; MoveToActiveSpace
    // brings it to the Space that is active instead of switching away. The two
    // are in different groups of the options, so they combine.
    // https://developer.apple.com/documentation/appkit/nswindow/collectionbehavior-swift.struct
    [window setCollectionBehavior:NSWindowCollectionBehaviorFullScreenAuxiliary |
                                  NSWindowCollectionBehaviorMoveToActiveSpace];

    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    // The default data store is the persistent one the launcher's own webview
    // uses. The card keeps nothing worth persisting, so it gets one that lives
    // in memory and writes nothing to disk. Apple: WKWebsiteDataStore
    // nonPersistent() (+nonPersistentDataStore in Objective-C).
    // https://developer.apple.com/documentation/webkit/wkwebsitedatastore/nonpersistent()
    config.websiteDataStore = [WKWebsiteDataStore nonPersistentDataStore];
    [config setURLSchemeHandler:splash forURLScheme:scheme];
    [config.userContentController addScriptMessageHandler:splash name:kMessageHandlerName];

    WKWebView *webView = [[WKWebView alloc] initWithFrame:window.contentView.bounds configuration:config];
    [webView setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    // Transparent over the window colour, so the card never flashes white. The
    // key is private but is what Wails uses; a WebKit without it just paints.
    @try {
        [webView setValue:@NO forKey:@"drawsBackground"];
    } @catch (NSException *exception) {
    }
    webView.navigationDelegate = splash;
    [window.contentView addSubview:webView];

    splash.window = window;
    splash.webView = webView;
    [webView loadRequest:[NSURLRequest requestWithURL:url]];
    return splash;
}

void *splashCreate(uintptr_t handle, const double *frame, const int *rgb, const char *scheme, const char *url) {
    NSString *schemeName = scheme ? [NSString stringWithUTF8String:scheme] : nil;
    NSURL *pageURL = url ? [NSURL URLWithString:[NSString stringWithUTF8String:url]] : nil;
    if (schemeName == nil || pageURL == nil) return NULL;
    NSRect rect = NSMakeRect(frame[0], frame[1], frame[2], frame[3]);
    NSColor *colour = [NSColor colorWithSRGBRed:rgb[0] / 255.0 green:rgb[1] / 255.0 blue:rgb[2] / 255.0 alpha:1.0];

    __block void *ref = NULL;
    BOOL ran = runOnMain(10.0, ^{
        KSplash *splash = nil;
        @try {
            splash = makeSplash(handle, rect, colour, schemeName, pageURL);
        } @catch (NSException *exception) {
            splash = nil;
        }
        if (splash != nil) ref = (__bridge_retained void *)splash;
    });
    return ran ? ref : NULL;
}

void splashShow(void *ref) {
    if (ref == NULL) return;
    KSplash *splash = (__bridge KSplash *)ref;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (splash.closed) return;
        [splash.window makeKeyAndOrderFront:nil];
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        // Deprecated in macOS 14, where it is the same as activate; kept for the
        // older systems and to match Wails' own Show.
        [NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
    });
}

void splashEval(void *ref, const char *stateJSON) {
    if (ref == NULL || stateJSON == NULL) return;
    KSplash *splash = (__bridge KSplash *)ref;
    NSString *state = [NSString stringWithUTF8String:stateJSON];
    if (state == nil) return;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (splash.closed) return;
        splash.pending = state;
        splash.retries = 0;
        [splash flush];
    });
}

int splashClose(void *ref) {
    if (ref == NULL) return 1;
    // The reference Go held is taken over here and released when this block's
    // scope ends, after the teardown. A block that never ran leaves it held.
    __block void *owned = ref;
    BOOL ran = runOnMain(5.0, ^{
        KSplash *splash = (__bridge_transfer KSplash *)owned;
        owned = NULL;
        [splash teardown];
    });
    return ran ? 1 : 0;
}

void splashPump(void) {
    @autoreleasepool {
        static BOOL started = NO;
        if (!started) {
            started = YES;
            [NSApplication sharedApplication];
            [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
            [NSApp finishLaunching];
        }
        [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.02]];
    }
}
