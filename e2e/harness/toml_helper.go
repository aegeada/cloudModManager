package harness

import (
	"strings"
)

func (r *RunResult) AssertLockfileMod(slug, version string, pinned bool) {
	r.T.Helper()
	lockfile := r.Ctx.ReadFile("cmm.lock")
	if !strings.Contains(lockfile, slug) {
		r.T.Fatalf("expected mod %s in lockfile", slug)
	}
}
