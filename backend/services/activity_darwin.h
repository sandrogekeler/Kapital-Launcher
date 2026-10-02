// An OS activity for the game tracker (activity_darwin.go): what Go calls.
//
// Plain C types only, because cgo copies this file into the Go preamble.
#ifndef KAPITAL_SERVICES_ACTIVITY_DARWIN_H
#define KAPITAL_SERVICES_ACTIVITY_DARWIN_H

// kapitalBeginActivity tells macOS the app is doing work the player asked for,
// which keeps App Nap from throttling it, and returns an owned token for
// kapitalEndActivity, or NULL. reason is a UTF-8 string macOS shows in Activity
// Monitor. The system may still sleep when the machine is idle.
void *kapitalBeginActivity(const char *reason);

// kapitalEndActivity ends the activity and releases the token, which must not
// be used again. NULL does nothing.
void kapitalEndActivity(void *token);

#endif
