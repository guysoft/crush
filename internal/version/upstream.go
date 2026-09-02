package version

import "fmt"

// Fork-only version metadata. These vars are set via -ldflags at release
// time by the fork's .goreleaser.fork.yml and stay untouched by the
// init() in version.go, which rewrites Version from debug.ReadBuildInfo().
var (
	// ForkVersion is the fork's own semver (e.g. "0.1.0"), including any
	// pre-release suffix from the tag. Empty for dev builds.
	ForkVersion = ""

	// UpstreamVersion is the charmbracelet/crush version this fork build
	// is based on (e.g. "0.89.0"). "unknown" for dev builds.
	UpstreamVersion = "unknown"
)

// Display returns the user-facing version string. When ForkVersion and
// UpstreamVersion are both set (release builds), it renders both; else
// it falls back to Version (which upstream's init() populates for dev
// builds).
func Display() string {
	if ForkVersion == "" || UpstreamVersion == "" || UpstreamVersion == "unknown" {
		return Version
	}
	return fmt.Sprintf("%s (based on charmbracelet/crush v%s)", ForkVersion, UpstreamVersion)
}
