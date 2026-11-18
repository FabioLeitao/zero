package buildlocal

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitIn runs git in dir with a hermetic configuration (no signing, no user
// config, a fixed identity).
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = hermeticEnv(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func hermeticEnv(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture builds an "upstream" repo tagged v1.0.0 and a fork clone one commit
// ahead of it that carries a single patch file, p/alpha.txt.
func fixture(t *testing.T) (work string, upstreamSHA8 string) {
	t.Helper()
	root := t.TempDir()
	up := filepath.Join(root, "up")
	work = filepath.Join(root, "work")
	for _, d := range []string{up, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, up, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(up, "a.txt"), "1\n")
	gitIn(t, up, "add", ".")
	gitIn(t, up, "commit", "-q", "-m", "one")
	gitIn(t, up, "tag", "v1.0.0")
	upstreamSHA8 = gitIn(t, up, "rev-parse", "--short=8", "HEAD")

	gitIn(t, work, "init", "-q", "-b", "main")
	gitIn(t, work, "remote", "add", "upstream", up)
	gitIn(t, work, "remote", "add", "origin", up)
	gitIn(t, work, "fetch", "-q", "--tags", "upstream")
	gitIn(t, work, "reset", "-q", "--hard", "upstream/main")
	writeFile(t, filepath.Join(work, "p", "alpha.txt"), "patch\n")
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-q", "-m", "alpha patch")
	return work, upstreamSHA8
}

// runScript runs the build script with the fork clone and label table given.
// It returns stdout, stderr and the exit code.
func runScript(t *testing.T, work, labels string, args ...string) (string, string, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "zero-databoar-build"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = append(hermeticEnv(t), "ZERO_REPO="+work, "ZERO_DATABOAR_LABELS="+labels)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run script: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func requireTools(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the build script is bash for Linux and macOS")
	}
	for _, tool := range []string{"bash", "git", "timeout"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
}

func TestBuildScriptVersionCarriesTheLabelOfAPatchStillOnlyOnTheFork(t *testing.T) {
	requireTools(t)
	work, upSHA := fixture(t)
	labels := filepath.Join(t.TempDir(), "labels.tsv")
	writeFile(t, labels, "alpha:\np/alpha.txt\n")
	refSHA := gitIn(t, work, "rev-parse", "--short=8", "HEAD")

	stdout, stderr, code := runScript(t, work, labels, "--ref", "main", "--print-version")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	want := "1.0.0+up.0." + upSHA + ".databoar." + refSHA + ".alpha"
	if got := strings.TrimSpace(stdout); got != want {
		t.Fatalf("version = %q, want %q", got, want)
	}
}

func TestBuildScriptRefusesAPathNoLabelOwnsAndCheckLabelsNamesIt(t *testing.T) {
	requireTools(t)
	work, _ := fixture(t)
	writeFile(t, filepath.Join(work, "other.txt"), "x\n")
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-q", "-m", "an unowned path")
	labels := filepath.Join(t.TempDir(), "labels.tsv")
	writeFile(t, labels, "alpha:\np/alpha.txt\n")

	if _, stderr, code := runScript(t, work, labels, "--ref", "main", "--print-version"); code != 4 {
		t.Fatalf("print-version exit = %d, want 4\n%s", code, stderr)
	}
	stdout, stderr, code := runScript(t, work, labels, "--ref", "main", "--check-labels")
	if code != 4 {
		t.Fatalf("check-labels exit = %d, want 4\n%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "other.txt" {
		t.Fatalf("check-labels stdout = %q, want the path to add (other.txt)", stdout)
	}

	// Once the table names the path (here as a process file), the same ref closes.
	writeFile(t, labels, "alpha:\np/alpha.txt\n\nignore:\nother.txt\n")
	if _, stderr, code := runScript(t, work, labels, "--ref", "main", "--check-labels"); code != 0 {
		t.Fatalf("check-labels exit = %d, want 0 after the table names the path\n%s", code, stderr)
	}
}

func TestBuildScriptFailsMainWhenALabelWasAbsorbedUpstream(t *testing.T) {
	requireTools(t)
	work, _ := fixture(t)
	labels := filepath.Join(t.TempDir(), "labels.tsv")
	// a.txt is identical on upstream and the fork: the label owns nothing the fork still carries.
	writeFile(t, labels, "alpha:\np/alpha.txt\nbeta:\na.txt\n")
	if _, stderr, code := runScript(t, work, labels, "--ref", "main", "--print-version"); code != 6 {
		t.Fatalf("exit = %d, want 6 (absorbed label)\n%s", code, stderr)
	}
}
