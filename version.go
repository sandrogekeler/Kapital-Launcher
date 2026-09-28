package main

// Version is the build version, overridden at release time with
// `-ldflags "-X main.Version=<tag>"`. The "-dev" suffix marks a build that is
// not a release: `wails dev`, or a plain `go build`. Mirror the base in
// wails.json's info.productVersion (.claude/rules/builds-and-releases.md).
var Version = "0.1.0-dev"
