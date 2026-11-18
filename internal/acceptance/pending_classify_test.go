package acceptance

import "testing"

// TestPendingClassification pins which child outputs count as an expected
// failure. The mixed cases failed before expectedFailure checked every error
// line: they were hidden as "pending".
func TestPendingClassification(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"assertion only", "    a_test.go:10: ACCEPT-FAIL: x\n--- FAIL: TestX\n", true},
		{"assertion in subtest", "    a_test.go:10: ACCEPT-FAIL: x\n    --- FAIL: TestX/sub\n", true},
		{"no marker", "    a_test.go:10: something\n--- FAIL: TestX\n", false},
		{"assertion then bare Fatal", "    a_test.go:10: ACCEPT-FAIL: x\n    a_test.go:11: unexpected\n--- FAIL: TestX\n", false},
		{"cleanup error after assertion", "    a_test.go:10: ACCEPT-FAIL: x\n    testing.go:1: TempDir RemoveAll cleanup: boom\n", false},
		{"setup failure", "    a_test.go:10: SETUP-FAIL: x\n    a_test.go:11: ACCEPT-FAIL: y\n", false},
		{"panic", "    a_test.go:10: ACCEPT-FAIL: x\npanic: boom\n", false},
	}
	for _, c := range cases {
		if got := expectedFailure(c.out); got != c.want {
			t.Errorf("%s: expectedFailure = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestConfigBases is a hermetic fake of the per-OS config lookup: on Windows
// zero reads %AppData%, not XDG_CONFIG_HOME.
func TestConfigBases(t *testing.T) {
	s := &sandbox{Config: "cfg", AppData: "app"}
	if got := s.configBases("linux"); len(got) != 1 || got[0] != "cfg" {
		t.Errorf("linux: %v", got)
	}
	if got := s.configBases("darwin"); len(got) != 1 || got[0] != "cfg" {
		t.Errorf("darwin: %v", got)
	}
	got := s.configBases("windows")
	if len(got) != 2 || got[0] != "cfg" || got[1] != "app" {
		t.Errorf("windows: %v", got)
	}
}
