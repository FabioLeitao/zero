package acceptance

// Pending marker for acceptance tests written before the task is implemented.
//
// This file is deliberately self-contained so it is easy to delete once no test
// calls pending() any more.
//
// pending(t, task, body) re-runs the single test in a child process:
//
//   - the child fails on an assertion (a line starting "ACCEPT-FAIL:"): the
//     failure is the expected one, and the test is skipped as "pending: task X";
//   - the child passes: the task is done, so the test FAILS with "marked
//     pending but passes: remove the marker" and the developer has to change
//     pending(t, "X", body) to a plain body(t) call;
//   - the child fails for any other reason (a setup error, a panic, a timeout,
//     a crash of the binary under test), or logs any failure line that is not an
//     "ACCEPT-FAIL:" one (a bare t.Fatal, a testing cleanup error): that is not
//     an expected failure, so the test FAILS. Every error line of the child must
//     carry the marker for the failure to count as expected.
//
// The equivalent in pytest is xfail(strict=True).

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

const pendingChildEnv = "ZERO_ACCEPTANCE_PENDING_CHILD"

func pending(t *testing.T, task string, body func(t *testing.T)) {
	t.Helper()
	if os.Getenv(pendingChildEnv) == "1" {
		body(t)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		setupFatal(t, "pending: locate test binary: %v", err)
	}
	parts := strings.Split(t.Name(), "/")
	for i, p := range parts {
		parts[i] = "^" + regexp.QuoteMeta(p) + "$"
	}
	cmd := exec.Command(exe, "-test.run", strings.Join(parts, "/"), "-test.count=1", "-test.v", "-test.timeout=5m")
	cmd.Env = append(os.Environ(), pendingChildEnv+"=1")
	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	text := string(out)
	passed := runErr == nil && strings.Contains(text, "--- PASS: "+t.Name())
	switch {
	case passed:
		t.Fatalf("marked pending but passes: remove the marker (task %s, test %s)", task, t.Name())
	case !expectedFailure(text):
		t.Fatalf("pending test failed for a reason other than the missing task %s (setup error, panic, timeout or crash):\n%s", task, text)
	default:
		t.Skipf("pending: task %s: %s (%s)", task, firstLineWith(text, "ACCEPT-FAIL:"), time.Since(start).Round(time.Second))
	}
}

// errorLine matches the log lines "file.go:12: message" the testing package
// prints for t.Error, t.Fatal and friends.
var errorLine = regexp.MustCompile(`^\s+\S+\.go:\d+: `)

// expectedFailure reports whether the output of a failed child is exactly the
// expected failure: at least one ACCEPT-FAIL line, and no setup error, panic,
// skip or error line without the marker.
func expectedFailure(text string) bool {
	if strings.Contains(text, "SETUP-FAIL:") || strings.Contains(text, "panic:") || strings.Contains(text, "--- SKIP") {
		return false
	}
	found := false
	for _, line := range strings.Split(text, "\n") {
		if !errorLine.MatchString(line) {
			continue
		}
		if !strings.Contains(line, "ACCEPT-FAIL:") {
			return false
		}
		found = true
	}
	return found
}

func firstLineWith(text, marker string) string {
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):])
		}
	}
	return ""
}

// expectf records an assertion failure of the behavior under acceptance. Only
// failures raised through it can make a pending test count as "expected".
func expectf(t testing.TB, ok bool, format string, args ...any) bool {
	t.Helper()
	if !ok {
		t.Errorf("ACCEPT-FAIL: "+format, args...)
	}
	return ok
}
