// Package version exposes the build-time-injected Version string used by
// the --version CLI flag.
package version

var Version = "dev" // overridden at build time via -ldflags
