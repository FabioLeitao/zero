---
name: project-acceptance-conventions
description: How the zero fork's black-box acceptance tests (internal/acceptance) are built, switched on and validated
metadata:
  type: project
---

Acceptance tests live in `internal/acceptance` (Go, real `zero` binary, fake provider in `harness_test.go`, sandbox env without LANG/LC_*, TZ=UTC). No `pending()` helper exists (the 2.1 one was removed after implementation).

Task 2.6 convention: one constant `activityImplemented` in `activity_test.go` skips every A2.6-* test; a strict guard test (`TestA2_6_0_PendingSwitch`) fails if the block exists while the constant is false (the XPASS(strict) analogue). `sandbox.Extra` (harness) adds env entries for one run (TZ, LC_ALL).

Event dates in tests are derived from the real `zero exec` timestamps ("anchor"), not from the test process clock; tests that need "today UTC" skip if midnight UTC passed.

**Why:** the operator wants the dev to flip one line; tests must compile and pass (by skip) before the implementation. **How to apply:** reuse this pattern for later tasks; prove each test with a throwaway reference implementation in a copy of the tree under the scratchpad (never write production files in the clone; the auto-mode classifier blocks it). `/tmp` is a git repo on this host, so many unrelated tests fail in `go test ./...` here (memory, sessions, tui...): rely on `zero-gate`.
