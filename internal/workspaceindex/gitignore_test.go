package workspaceindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// isolateGitConfig keeps the git child process gitIgnoredPaths spawns away from the
// developer's real global/system config — a global core.excludesFile would otherwise change
// what counts as ignored and make these tests depend on the machine they run on.
// GIT_CONFIG_GLOBAL needs git >= 2.32; HOME/XDG_CONFIG_HOME cover older git too.
func isolateGitConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// initGitRepoWithGitignore turns root into a real git work tree with the given .gitignore,
// so gitIgnoredPaths has something to resolve. Skips (never fails) when git is unavailable:
// the gitignore layer is best-effort by design and ShouldSkipDir coverage is independent.
func initGitRepoWithGitignore(t *testing.T, root, gitignore string) {
	t.Helper()
	isolateGitConfig(t)
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Skipf("git unavailable, skipping gitignore-aware coverage: %v: %s", err, out)
	}
	writeFile(t, root, ".gitignore", gitignore)
}

// Regression: a venv/build-cache dir with a name no fixed list anticipates must still be
// skipped when the repo's own .gitignore excludes it. Reported against a real repo where
// `zero repo-map` burned its whole scan budget inside a `.locust_env/` virtualenv (the
// `locust` load-testing tool's naming, not `.venv`/`venv`) and never reached the repo's
// own README.md. ShouldSkipDir can never cover this — it is a closed list by design.
func TestScanHonorsRepoGitignore(t *testing.T) {
	root := t.TempDir()
	initGitRepoWithGitignore(t, root, ".locust_env/\n*.db\ncafé-cache/\n")

	writeFile(t, root, "README.md", "# Example\n")
	writeFile(t, root, "app/main.py", "print('hi')\n")
	writeFile(t, root, "app/café.py", "x = 1\n") // non-ASCII, NOT ignored: must survive
	// Ignored by .gitignore only — none of these names are in ShouldSkipDir/ShouldSkipFile.
	writeFile(t, root, ".locust_env/lib/python3.13/site-packages/flask/sansio/README.md", "vendored")
	writeFile(t, root, "audit_results.db", "sqlite")
	writeFile(t, root, "app/cache.db", "sqlite")
	writeFile(t, root, "café-cache/blob.json", "{}") // non-ASCII ignored dir (core.quotePath case)

	got, err := Scan(root, Options{MaxDepth: DefaultMaxDepth})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{".gitignore", "README.md", "app/café.py", "app/main.py"}
	if !reflect.DeepEqual(pathsOf(got.Files), want) {
		t.Fatalf("Files=%v want %v", pathsOf(got.Files), want)
	}
	for _, imp := range got.Files {
		if imp.Important && imp.Path != "README.md" {
			t.Fatalf("only the repo's own README.md should be important, got %q", imp.Path)
		}
	}
}

// Scanning a subdirectory of a repo must resolve ignore rules relative to that subdirectory,
// the same `rel` form Scan computes — not relative to the repository's top level.
func TestScanHonorsRepoGitignoreFromSubdirectoryRoot(t *testing.T) {
	repo := t.TempDir()
	initGitRepoWithGitignore(t, repo, "build-out/\n")
	writeFile(t, repo, "svc/handler.go", "package svc\n")
	writeFile(t, repo, "svc/build-out/gen.go", "package gen\n")

	got, err := Scan(filepath.Join(repo, "svc"), Options{MaxDepth: DefaultMaxDepth})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if want := []string{"handler.go"}; !reflect.DeepEqual(pathsOf(got.Files), want) {
		t.Fatalf("Files=%v want %v", pathsOf(got.Files), want)
	}
}

// Outside a git work tree the layer must be a silent no-op: no error, and Scan keeps the
// exact pre-existing behavior (fixed denylist only).
func TestGitIgnoredPathsOutsideGitRepoFallsBack(t *testing.T) {
	isolateGitConfig(t)
	root := t.TempDir()
	if paths, ok := gitIgnoredPaths(root); ok || paths != nil {
		t.Fatalf("gitIgnoredPaths(non-repo) = (%v, %v), want (nil, false)", paths, ok)
	}

	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, ".locust_env/lib/x.py", "kept: no git, no .gitignore to honor")
	writeFile(t, root, "node_modules/pkg/index.js", "still skipped by the fixed denylist")
	got, err := Scan(root, Options{MaxDepth: DefaultMaxDepth})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if want := []string{".locust_env/lib/x.py", "main.go"}; !reflect.DeepEqual(pathsOf(got.Files), want) {
		t.Fatalf("Files=%v want %v", pathsOf(got.Files), want)
	}
}
