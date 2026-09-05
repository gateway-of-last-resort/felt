package version

import (
	"strings"
	"testing"
)

// A stamped build reports its tag, with exactly one leading v.
func TestStampedVersion(t *testing.T) {
	defer restore(Version, Commit, Date)

	Version = "1.2.3"
	if got := String(); got != "v1.2.3" {
		t.Errorf("String() = %q, want v1.2.3", got)
	}

	Version = "v1.2.3"
	if got := String(); got != "v1.2.3" {
		t.Errorf("String() = %q, want the v not doubled", got)
	}
}

// An unstamped build says so rather than claiming a version it does not have.
func TestUnstampedVersion(t *testing.T) {
	defer restore(Version, Commit, Date)

	Version = ""
	got := String()
	if !strings.HasPrefix(got, "dev") {
		t.Errorf("String() = %q, want it to admit to being a dev build", got)
	}
}

// --version carries enough to identify a build in a bug report.
func TestFull(t *testing.T) {
	defer restore(Version, Commit, Date)

	Version, Commit, Date = "1.0.0", "0123456789abcdef", "2026-09-05"
	got := Full()

	for _, want := range []string{"felt", "v1.0.0", "0123456", "2026-09-05"} {
		if !strings.Contains(got, want) {
			t.Errorf("Full() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "0123456789abcdef") {
		t.Error("Full() prints the whole commit hash, want it shortened")
	}
}

func restore(v, c, d string) { Version, Commit, Date = v, c, d }
