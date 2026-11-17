# sbxcompanion (rascunho de Discussion em Ideas, NÃO abrir)

Categoria: Discussions > Ideas (é melhoria, não bug: sem o helper o `zero` usa a si mesmo, como o README descreve). Abrir só com o "vai" do operador e depois que a #1229 e a #1148 andarem.

**Title:** `make build` could also build the Linux sandbox helper

**Problem:** On Linux, `make build` produces only `zero`. The sandbox helper (`zero-linux-sandbox`) is a separate manual `go build` step that the docs call optional. A contributor who builds from source and runs the sandboxed tools can end up with the helper missing next to the binary without noticing.

**Idea:** On Linux, make the `build` target also build `./cmd/zero-linux-sandbox` next to `zero`, and make `clean` remove it. Other platforms keep the current behavior. The release tooling already builds the helper, so this only aligns the local build with it.

**Alternatives:** Keep it manual and add a `make build-sandbox` target; or a note in `CONTRIBUTING.md`.

**Question for maintainers:** Is this wanted, and which of the three shapes do you prefer? We are happy to prepare the change once an issue is approved.
