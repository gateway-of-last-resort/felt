// Package version carries the build's identity.
//
// The values are stamped in at link time by GoReleaser, so a release binary
// knows what it is without anyone editing a constant. A build made with plain
// `go build` keeps the defaults and says so.
package version

import "runtime/debug"

// Stamped by the linker: see the ldflags in .goreleaser.yaml.
var (
	Version = ""
	Commit  = ""
	Date    = ""
)

// Repo is where the project lives, shown in the menu so that a player who
// found a bug knows where to take it.
const Repo = "github.com/gateway-of-last-resort"

// String is the version as a player should see it: "v1.2.3" for a release,
// or a short commit for anything else.
//
// A build from source has no linker stamp, so it falls back to the revision
// Go records in the binary itself — which is more useful in a bug report than
// the word "unknown".
func String() string {
	if Version != "" {
		return withV(Version)
	}
	if rev := vcsRevision(); rev != "" {
		return "dev-" + rev
	}
	return "dev"
}

// Full adds the commit and build date, for --version.
func Full() string {
	out := "felt " + String()
	if Commit != "" {
		out += " (" + short(Commit) + ")"
	}
	if Date != "" {
		out += " built " + Date
	}
	return out
}

func withV(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v
	}
	return "v" + v
}

func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// vcsRevision reads the commit Go stamps into every binary built from a git
// checkout.
func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return short(s.Value)
		}
	}
	return ""
}
