package workspaceindex

import (
	"bytes"
	"context"
	"os/exec"
	"path"
	"strings"
	"time"
)

// gitIgnoredPathsTimeout bounds how long Scan waits on git before giving up and falling
// back to the static denylist alone. Large repos with many ignored files (vendored
// node_modules, build caches) still resolve well under this on local disk; a slow/degenerate
// case (network filesystem, huge monorepo) should not hang the whole scan.
const gitIgnoredPathsTimeout = 5 * time.Second

// gitIgnoredPaths asks git which paths under root it considers ignored — honoring the real
// .gitignore (nested files, .git/info/exclude, core.excludesFile, everything git itself
// already resolves), not a hardcoded name list. Returns a set of paths relative to root —
// whole ignored directories collapsed to one entry, plus individually ignored files — using
// forward slashes and no trailing slash (matching the `rel` form Scan already computes).
//
// Returns (nil, false) when root is not inside a git work tree, git is unavailable, or a
// command times out/fails for any other reason — callers fall back to ShouldSkipDir and
// ShouldSkipFile alone in every one of those cases. This is deliberately best-effort: a scan
// must never fail or hang because of this.
//
// Why `git status --ignored=matching` and not `git ls-files --others --ignored --directory`:
// ls-files stops descending at the first untracked directory, so an ignored directory or file
// nested inside one (svc/build-out/, app/cache.db) is never reported. status descends,
// collapses a fully-ignored directory to a single entry, and lists partially-ignored files
// individually — the granularity Scan needs.
func gitIgnoredPaths(root string) (map[string]bool, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitIgnoredPathsTimeout)
	defer cancel()

	// status reports paths relative to the repository top level even when run from a
	// subdirectory, while Scan's `rel` is relative to root. show-prefix is root's own
	// location inside the repo ("" at the top level, "svc/" for <repo>/svc), and also the
	// cheapest way to learn this is a git work tree at all.
	prefixOut, err := gitOutput(ctx, root, "rev-parse", "--show-prefix")
	if err != nil {
		// Covers: not a git repo, git missing from PATH, deadline exceeded. None of these
		// are errors Scan should propagate.
		return nil, false
	}
	prefix := strings.TrimSpace(string(prefixOut))

	// --porcelain=v1: stable, script-oriented format. -z: NUL-terminated and unquoted —
	// without it git C-quotes non-ASCII/special bytes (core.quotePath), e.g. "caf\303\251/",
	// which would never match the raw `rel` Scan computes. --ignored=matching: list ignored
	// paths, collapsing a directory that is ignored as a whole. The trailing "." limits the
	// report to root even when root is a subdirectory of the repository.
	out, err := gitOutput(ctx, root, "status", "--porcelain=v1", "-z", "--ignored=matching", ".")
	if err != nil {
		return nil, false
	}

	paths := make(map[string]bool)
	for entry := range bytes.SplitSeq(out, []byte{0}) {
		// Ignored entries are "!! <path>"; everything else (untracked "??", changes,
		// renames' second NUL field) is not ours and is skipped.
		rest, ok := bytes.CutPrefix(entry, []byte("!! "))
		if !ok {
			continue
		}
		rel, ok := strings.CutPrefix(string(rest), prefix)
		if !ok {
			continue
		}
		// git prints a trailing "/" for directory entries; Scan's own `rel` never carries one.
		rel = path.Clean(strings.TrimSuffix(rel, "/"))
		if rel == "." || rel == "" {
			continue
		}
		paths[rel] = true
	}
	return paths, true
}

// gitOutput runs git in dir with a fixed argv list (no shell). dir is the caller's own
// absolute path and the arguments are constants, so there is nothing to inject.
func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	// #nosec G204 -- fixed argv, no shell; dir is the caller's own absolute path.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	return cmd.Output()
}
