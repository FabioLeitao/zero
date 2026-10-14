package acp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Gitlawb/zero/internal/agent"
	"github.com/Gitlawb/zero/internal/sessions"
	"github.com/Gitlawb/zero/internal/zeroruntime"
)

// `zero exec` persists every provider usage report as a session usage event, so
// `zero usage report --session <id>` can total a run. The ACP agent did not set
// OnUsage, which dropped the token counts of every ACP session. A turn with two
// provider requests (a tool round trip) must leave two usage events, in the shape
// the usage report reads, next to the turn's messages.
func TestACPPromptPersistsProviderUsage(t *testing.T) {
	deps := testDeps(t)
	deps.RunAgent = func(_ context.Context, _ string, _ zeroruntime.Provider, opts agent.Options) (agent.Result, error) {
		if opts.OnUsage == nil {
			t.Errorf("agent.Options.OnUsage is not wired: the ACP agent drops provider token usage")
			return agent.Result{FinalAnswer: "done"}, nil
		}
		opts.OnUsage(agent.Usage{InputTokens: 1200, OutputTokens: 34})
		opts.OnUsage(agent.Usage{InputTokens: 1300, OutputTokens: 21, CachedInputTokens: 900})
		return agent.Result{FinalAnswer: "done"}, nil
	}

	h := newHarness(t, deps)
	defer h.stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var created NewSessionResult
	if err := h.client.Call(ctx, MethodSessionNew, NewSessionParams{Cwd: t.TempDir()}, &created); err != nil {
		t.Fatal(err)
	}
	var result PromptResult
	if err := h.client.Call(ctx, MethodSessionPrompt, PromptParams{
		SessionID: created.SessionID, Prompt: []ContentBlock{TextBlock("count my tokens")},
	}, &result); err != nil {
		t.Fatalf("session/prompt: %v", err)
	}

	events, err := deps.Store.ReadEvents(created.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	type usagePayload struct {
		PromptTokens      int `json:"promptTokens"`
		CompletionTokens  int `json:"completionTokens"`
		TotalTokens       int `json:"totalTokens"`
		CachedInputTokens int `json:"cachedInputTokens"`
	}
	var got []usagePayload
	messages := 0
	for _, event := range events {
		switch event.Type {
		case sessions.EventUsage:
			var payload usagePayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("usage payload %s: %v", event.Payload, err)
			}
			got = append(got, payload)
		case sessions.EventMessage:
			messages++
		}
	}
	want := []usagePayload{
		{PromptTokens: 1200, CompletionTokens: 34, TotalTokens: 1234},
		{PromptTokens: 1300, CompletionTokens: 21, TotalTokens: 1321, CachedInputTokens: 900},
	}
	if len(got) != len(want) {
		t.Fatalf("usage events = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("usage event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if messages != 2 {
		t.Fatalf("message events = %d, want the user and assistant messages of the turn", messages)
	}
}

// Usage joins the turn's buffered batch, so a hard run failure that aborts the
// turn leaves nothing durable, exactly as it does for the turn's messages.
func TestACPHardTurnFailureDropsItsUsageWithTheTurn(t *testing.T) {
	deps := testDeps(t)
	deps.RunAgent = func(_ context.Context, _ string, _ zeroruntime.Provider, opts agent.Options) (agent.Result, error) {
		if opts.OnUsage == nil {
			t.Errorf("agent.Options.OnUsage is not wired: the ACP agent drops provider token usage")
			return agent.Result{}, errors.New("injected hard run failure")
		}
		opts.OnUsage(agent.Usage{InputTokens: 500, OutputTokens: 5})
		return agent.Result{}, errors.New("injected hard run failure")
	}

	h := newHarness(t, deps)
	defer h.stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var created NewSessionResult
	if err := h.client.Call(ctx, MethodSessionNew, NewSessionParams{Cwd: t.TempDir()}, &created); err != nil {
		t.Fatal(err)
	}
	if err := h.client.Call(ctx, MethodSessionPrompt, PromptParams{
		SessionID: created.SessionID, Prompt: []ContentBlock{TextBlock("failing turn")},
	}, &PromptResult{}); err == nil {
		t.Fatal("hard RunAgent failure was reported as a successful turn")
	}
	events, err := deps.Store.ReadEvents(created.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("aborted turn left durable events: %+v", events)
	}
}
