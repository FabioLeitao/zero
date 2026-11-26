---
name: project-zero-fragile-points
description: Pontos frágeis do fork zero para desenhos - usage.go compartilhado entre rótulos, TZ no Windows, hook pre-commit sem config
metadata:
  type: project
---

- `internal/cli/usage.go` é do rótulo `rmodel`, e o desenho 2.6 prevê nele o gancho do `ustats`: se isso for implementado, enquanto os dois existirem, o `rmodel` não some sozinho quando o upstream absorver (saída 6 do build não dispara). Ao absorver, tirar o arquivo da seção `rmodel`.
  **Why:** o script de rótulos compara caminhos, não patches. **How to apply:** em todo desenho que toque arquivo de um rótulo, dizer em qual seção ele fica e o que fazer quando o upstream absorver.
- Go ignora `TZ` no Windows (só `zoneinfo_unix.go` lê) e não entende `TZ` POSIX (`<-03>3` vira UTC); usar `Etc/GMT+3`. O sandbox de `internal/acceptance` põe `TZ=UTC` e nenhum `LANG`.
  **How to apply:** testes A que dependem do dia ou do fuso: só Linux/macOS, ou só números independentes do fuso.
- (Detalhe do clone desta máquina, em 2026-11-18) hook `pre-commit` instalado sem `.pre-commit-config.yaml`: o commit pede `PRE_COMMIT_ALLOW_NO_CONFIG=1`.
- `build/patch-labels.tsv` nasceu na PR #43 (build/ritual-20261118); conferir se já está no `main` antes de citar.

Ver [[feedback-escapes-invisiveis]].
