package intruder

import (
	"testing"

	"github.com/lea-151107/pollen/internal/userconfig"
)

// userconfigSetOverride redirects userconfig.Dir to a temp directory for
// the duration of the test so SaveLastRun / LoadLastRun don't touch the
// developer's real ~/.config/pollen.
func userconfigSetOverride(t *testing.T, dir string) {
	t.Helper()
	userconfig.SetOverride(dir)
	t.Cleanup(func() { userconfig.SetOverride("") })
}
