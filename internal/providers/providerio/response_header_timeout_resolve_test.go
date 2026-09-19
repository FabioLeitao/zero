package providerio

import (
	"testing"
	"time"
)

func TestResolveResponseHeaderTimeout(t *testing.T) {
	const env = "ZERO_RESPONSE_HEADER_TIMEOUT"

	t.Run("default when env is unset", func(t *testing.T) {
		t.Setenv(env, "")
		if got := resolveResponseHeaderTimeout(); got != defaultResponseHeaderTimeout {
			t.Fatalf("got %v, want default %v", got, defaultResponseHeaderTimeout)
		}
	})

	t.Run("env Go duration", func(t *testing.T) {
		t.Setenv(env, "240s")
		if got := resolveResponseHeaderTimeout(); got != 240*time.Second {
			t.Fatalf("got %v, want 240s", got)
		}
		t.Setenv(env, "5m")
		if got := resolveResponseHeaderTimeout(); got != 5*time.Minute {
			t.Fatalf("got %v, want 5m", got)
		}
	})

	t.Run("env bare seconds", func(t *testing.T) {
		t.Setenv(env, "300")
		if got := resolveResponseHeaderTimeout(); got != 300*time.Second {
			t.Fatalf("got %v, want 300s", got)
		}
	})

	t.Run("invalid env falls back to default", func(t *testing.T) {
		t.Setenv(env, "banana")
		if got := resolveResponseHeaderTimeout(); got != defaultResponseHeaderTimeout {
			t.Fatalf("got %v, want default %v on a typo", got, defaultResponseHeaderTimeout)
		}
	})

	t.Run("zero or negative env falls back to default", func(t *testing.T) {
		for _, value := range []string{"0", "-5s", "-1"} {
			t.Setenv(env, value)
			if got := resolveResponseHeaderTimeout(); got != defaultResponseHeaderTimeout {
				t.Fatalf("%q: got %v, want default %v (non-positive must not disable the watchdog)", value, got, defaultResponseHeaderTimeout)
			}
		}
	})

	t.Run("default matches upstream's original hardcoded value", func(t *testing.T) {
		if defaultResponseHeaderTimeout != 120*time.Second {
			t.Fatalf("default changed to %v; this env var is meant to add flexibility without changing upstream's chosen default", defaultResponseHeaderTimeout)
		}
	})
}
