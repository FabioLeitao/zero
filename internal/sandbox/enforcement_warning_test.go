package sandbox

import (
	"strings"
	"testing"
)

func TestEnforcementWarningUsesManagerDecision(t *testing.T) {
	t.Setenv(EnvSandboxed, "")
	t.Setenv(EnvSandboxBackend, "")
	workspace := t.TempDir()

	t.Run("degraded", func(t *testing.T) {
		engine := NewEngine(EngineOptions{
			WorkspaceRoot: workspace,
			Policy:        DefaultPolicy(),
			Backend: Backend{
				Name:     BackendUnavailable,
				Platform: "linux",
				Message:  "Linux sandbox helper is not available",
			},
		})

		level, reason := engine.EnforcementStatus()
		if level != EnforcementDegraded || reason != "Linux sandbox helper is not available" {
			t.Fatalf("EnforcementStatus() = %q, %q; want degraded with helper reason", level, reason)
		}
		warning := EnforcementWarning(engine)
		for _, want := range []string{"DEGRADED", reason, "zero doctor"} {
			if !strings.Contains(warning, want) {
				t.Fatalf("EnforcementWarning() = %q, want %q", warning, want)
			}
		}
	})

	t.Run("native", func(t *testing.T) {
		engine := NewEngine(EngineOptions{
			WorkspaceRoot: workspace,
			Policy:        DefaultPolicy(),
			Backend: Backend{
				Name:       BackendLinuxBwrap,
				Available:  true,
				Platform:   "linux",
				Executable: "/usr/bin/zero-linux-sandbox",
			},
		})
		if warning := EnforcementWarning(engine); warning != "" {
			t.Fatalf("EnforcementWarning() = %q, want silence for native enforcement", warning)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		policy := DefaultPolicy()
		policy.Mode = ModeDisabled
		engine := NewEngine(EngineOptions{
			WorkspaceRoot: workspace,
			Policy:        policy,
			Backend: Backend{
				Name:     BackendUnavailable,
				Platform: "linux",
			},
		})
		if warning := EnforcementWarning(engine); warning != "" {
			t.Fatalf("EnforcementWarning() = %q, want silence for explicitly disabled sandbox", warning)
		}
	})
}
