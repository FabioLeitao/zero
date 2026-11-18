// Package acceptance holds the black-box acceptance tests of the fork. They
// talk to the system the way a client does: the real `zero` process, a fake
// OpenAI-compatible provider on 127.0.0.1, the session event file and the
// `zero usage report` output. They import nothing from the application.
//
// Run (the binary is built from ../../cmd/zero; set ZERO_ACCEPTANCE_BIN to reuse
// a prebuilt one):
//
//	go test -count=1 ./internal/acceptance/
//
// Tests wrapped in pending(t, "<task>", ...) are expected failures until the
// task is implemented (see pending_test.go).
package acceptance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// binEnv lets a caller (CI, a developer) reuse a prebuilt binary.
const binEnv = "ZERO_ACCEPTANCE_BIN"

var zeroBin string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if p := os.Getenv(binEnv); p != "" {
		zeroBin = p
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "zero-acceptance-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "SETUP-FAIL: temp dir:", err)
		return 2
	}
	defer os.RemoveAll(dir)
	name := "zero"
	if runtime.GOOS == "windows" {
		name = "zero.exe"
	}
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, "../../cmd/zero")
	if b, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "SETUP-FAIL: go build ./cmd/zero: %v\n%s", err, b)
		return 2
	}
	zeroBin = out
	// Children of pending() reuse this binary instead of rebuilding it.
	os.Setenv(binEnv, out)
	return m.Run()
}

// setupFatal reports a failure of the test preparation. It is never an
// "expected failure": pending() refuses to treat it as one.
func setupFatal(t testing.TB, format string, args ...any) {
	t.Helper()
	t.Fatalf("SETUP-FAIL: "+format, args...)
}

// sandbox is an isolated user environment for one test.
type sandbox struct {
	t       *testing.T
	Home    string
	Data    string
	Config  string
	Cache   string
	Work    string
	AppData string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	s := &sandbox{
		t:       t,
		Home:    filepath.Join(root, "home"),
		Data:    filepath.Join(root, "data"),
		Config:  filepath.Join(root, "config"),
		Cache:   filepath.Join(root, "cache"),
		Work:    filepath.Join(root, "work"),
		AppData: filepath.Join(root, "appdata"),
	}
	for _, d := range []string{s.Home, s.Data, s.Config, s.Cache, s.Work, s.AppData} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			setupFatal(t, "mkdir %s: %v", d, err)
		}
	}
	return s
}

// env is the whole environment of the child process: nothing is inherited
// except what is needed to find tools, so the real user state is never read.
func (s *sandbox) env() []string {
	env := []string{
		"HOME=" + s.Home,
		"USERPROFILE=" + s.Home,
		"XDG_DATA_HOME=" + s.Data,
		"XDG_CONFIG_HOME=" + s.Config,
		"XDG_CACHE_HOME=" + s.Cache,
		"XDG_STATE_HOME=" + filepath.Join(s.Home, "state"),
		"APPDATA=" + s.AppData,
		"LOCALAPPDATA=" + s.AppData,
		"TMPDIR=" + s.Cache,
		"TEMP=" + s.Cache,
		"TMP=" + s.Cache,
		"TZ=UTC",
		"ZERO_DISABLE_MODELS_FETCH=1",
		"NO_PROXY=127.0.0.1,localhost",
		"no_proxy=127.0.0.1,localhost",
	}
	for _, k := range []string{"PATH", "SYSTEMROOT", "SystemRoot", "COMSPEC", "PATHEXT"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// writeConfig writes the user config with one OpenAI-compatible profile that
// points at the fake provider.
func (s *sandbox) writeConfig(profile map[string]any) {
	s.t.Helper()
	cfg := map[string]any{
		"activeProvider": profile["name"],
		"providers":      []any{profile},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		setupFatal(s.t, "marshal config: %v", err)
	}
	for _, base := range s.configBases(runtime.GOOS) {
		dir := filepath.Join(base, "zero")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			setupFatal(s.t, "mkdir config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), b, 0o600); err != nil {
			setupFatal(s.t, "write config: %v", err)
		}
	}
}

// configBases lists the base directories where zero may look for the user
// config on goos: XDG_CONFIG_HOME on Linux and macOS, and also %AppData% on
// Windows, where os.UserConfigDir ignores XDG_CONFIG_HOME
// (internal/config/paths.go, UserConfigDir).
func (s *sandbox) configBases(goos string) []string {
	if goos == "windows" {
		return []string{s.Config, s.AppData}
	}
	return []string{s.Config}
}

type result struct {
	Stdout, Stderr string
	Code           int
}

// zero runs the real binary in the sandbox.
func (s *sandbox) zero(args ...string) result {
	s.t.Helper()
	cmd := exec.Command(zeroBin, args...)
	cmd.Dir = s.Work
	cmd.Env = s.env()
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		setupFatal(s.t, "start zero: %v", err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				setupFatal(s.t, "run zero: %v", err)
			}
		}
		return result{out.String(), errb.String(), code}
	case <-time.After(90 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		setupFatal(s.t, "zero %v timed out\nstderr: %s", args, errb.String())
		return result{}
	}
}

// usageEvent is one decoded provider_usage line of events.jsonl.
type usageEvent struct {
	Line    string // the raw line, exactly as stored
	Payload map[string]json.RawMessage
}

// usageEvents reads the provider_usage events of a session from the file a
// client would read.
func (s *sandbox) usageEvents(sessionID string) []usageEvent {
	s.t.Helper()
	path := filepath.Join(s.Data, "zero", "sessions", sessionID, "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		setupFatal(s.t, "open events file: %v", err)
	}
	defer f.Close()
	var events []usageEvent
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			var env struct {
				Type    string                     `json:"type"`
				Payload map[string]json.RawMessage `json:"payload"`
			}
			if jerr := json.Unmarshal([]byte(line), &env); jerr == nil && env.Type == "provider_usage" {
				events = append(events, usageEvent{Line: line, Payload: env.Payload})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			setupFatal(s.t, "read events file: %v", err)
		}
	}
	return events
}

// fakeProvider is an OpenAI-compatible chat-completions server on loopback.
type fakeProvider struct {
	Server *httptest.Server
	// ModelLiteral is the JSON string literal (with quotes) put in the "model"
	// key of the chunks. Empty means the key is omitted.
	ModelLiteral string
	// ModelOnEveryChunk puts the model on the content chunks too, not only on
	// the usage chunk.
	ModelOnEveryChunk  bool
	Prompt, Completion int
	// CreatedModelLiteral is put in response.created (Responses API only) to
	// check that a non-terminal event is not taken as the answering model.
	CreatedModelLiteral string

	mu       sync.Mutex
	requests []map[string]any
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	p := &fakeProvider{Prompt: 11, Completion: 7}
	p.Server = httptest.NewServer(http.HandlerFunc(p.handle))
	t.Cleanup(p.Server.Close)
	return p
}

func (p *fakeProvider) URL() string { return p.Server.URL + "/v1" }

// RequestedModels returns the "model" sent in each chat-completions request.
func (p *fakeProvider) RequestedModels() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var models []string
	for _, r := range p.requests {
		if m, ok := r["model"].(string); ok {
			models = append(models, m)
		}
	}
	return models
}

func (p *fakeProvider) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/chat/completions"):
		p.handleChat(w, r)
	case strings.HasSuffix(r.URL.Path, "/responses"):
		p.handleResponses(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (p *fakeProvider) record(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	_ = json.Unmarshal(body, &req)
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
}

// handleChat answers a chat-completions request with an SSE stream.
func (p *fakeProvider) handleChat(w http.ResponseWriter, r *http.Request) {
	p.record(r)
	modelKey := func(on bool) string {
		if on && p.ModelLiteral != "" {
			return `"model":` + p.ModelLiteral + `,`
		}
		return ""
	}
	w.Header().Set("Content-Type", "text/event-stream")
	chunks := []string{
		fmt.Sprintf(`{"id":"c1","object":"chat.completion.chunk","created":1,%s"choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":null}]}`, modelKey(p.ModelOnEveryChunk)),
		fmt.Sprintf(`{"id":"c1","object":"chat.completion.chunk","created":1,%s"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`, modelKey(p.ModelOnEveryChunk)),
		fmt.Sprintf(`{"id":"c1","object":"chat.completion.chunk","created":1,%s"choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`, modelKey(true), p.Prompt, p.Completion, p.Prompt+p.Completion),
	}
	for _, c := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", c)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

// handleResponses answers a Responses-API request (the Codex backend) with SSE.
func (p *fakeProvider) handleResponses(w http.ResponseWriter, r *http.Request) {
	p.record(r)
	model := func(lit string) string {
		if lit == "" {
			return ""
		}
		return `"model":` + lit + `,`
	}
	w.Header().Set("Content-Type", "text/event-stream")
	events := []struct{ name, data string }{
		{"response.created", fmt.Sprintf(`{"type":"response.created","response":{"id":"r1",%s"status":"in_progress"}}`, model(p.CreatedModelLiteral))},
		{"response.output_text.delta", `{"type":"response.output_text.delta","item_id":"m1","output_index":0,"delta":"ok"}`},
		{"response.completed", fmt.Sprintf(`{"type":"response.completed","response":{"id":"r1",%s"status":"completed","usage":{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d}}}`, model(p.ModelLiteral), p.Prompt, p.Completion, p.Prompt+p.Completion)},
	}
	for _, e := range events {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, e.data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
