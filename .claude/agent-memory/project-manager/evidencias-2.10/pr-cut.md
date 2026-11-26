## What changed

Cut requested by the upstream maintainer in the review of Twigpine/zero#1234 (2026-11-19). The branch goes back to the shape of `58916070` for the provider and the collector, and the test names lose the `D2_1_` prefix.

- `bdc4f833` refactor(usage): drop the held usage event and the collector drain. Restores `provider.go`, `tool_state.go` and `zeroruntime/helpers.go` to `58916070` and removes `provider_late_model_test.go` and `collect_cancel_test.go`. `git diff 58916070 HEAD` is empty for these paths.
- `41b2120a` test(usage): drop the `D2_1_` prefix from the resolved model test names. 28 functions in 9 files, names only, no logic change.

Out of scope on purpose (maintainer): a provider that changes the model after the last usage chunk. It needs its own issue with a captured stream.

Base is `usage/1148-resolved-model` (the head of Twigpine/zero#1234). Nothing is pushed to that branch or to the upstream by this PR.

## Why

The held usage event and the 50 ms collector wait served one case that no known provider produces, and they changed the behavior of every provider (about 700 of the 2,667 lines).

## Verification

- `zero-gate` on `41b2120aff7d39f9eb458e1a62135da78054767d`: `ZERO_GATE_EXIT=3`. Exit 3 means the only failure is the known `lint-static` baseline (6 findings, none new). fmt-check, vet, test-race (148 s), test, build, smoke, vulncheck, vulncheck-windows and diff-check all PASS. It is not exit 0.
- The 28 renames match the sed rule applied to `HEAD` byte for byte; no `D2_1_` is left under `internal/`.
- Local build and dogfood on the fork `main` with this change applied (`candidate/2.10`, local ref): `zero-databoar-build` exit 0, version `0.14.0+up.0.8d23195f.databoar.7f041a85.rmodel.sbxcompanion.acphard.ustats`, smoke ok, govulncheck clean. In the TUI, `zero usage report --session` showed `auto -> poolside/laguna-xs-2.1` over 5 requests, and the tool calls worked (4 of 4 ok).
- Known gap, not fixed here: two mutations of `ModelReported` (`provider.go:377` and `codex_responses.go:907`) still pass the package tests. Two regression tests are proposed as a separate task.

Refs #53

— project-manager
