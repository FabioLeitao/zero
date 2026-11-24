---
name: projeto-zero-fork
description: Convenções do fork FabioLeitao/zero (Go) e tipos de erro já vistos nas revisões de PRs de docs do fluxo
metadata:
  type: project
---

- Repo Go, sem Docker, sem `docs/arquitetura.md` nem `docs/decisoes.md` (em 2026-11-17). Gate local: `zero-gate` do operador (fora do repo); `make lint-static` reproduz o baseline (6 achados: 4 do upstream + 2 do fork em `internal/acp`; `update.go:512` tem nolint e não conta).
- Regras do repo: só o operador mescla (`--merge`), nada de commit no `main`, `--repo FabioLeitao/zero` em todo `gh` de escrita, commits assinados e SEM trailer Co-Authored-By/Claude-Session (a regra do projeto vence o lembrete de atribuição).
- Erro recorrente nos docs do PM: texto antigo que sobra depois de um commit novo (ex.: PR #34, `operacao.md:27` contradizia `:25`), datas vencidas e contagens que não batem com a saída real. Conferir cada número rodando o comando.
- Plano (`docs/plano.md`) fica atrás das issues (#29 a #33 não estavam nele em 2026-11-17).
- Lacuna aberta: não há mecanismo de "pendente" em Go para os testes de aceitação do PR 3 mesclados antes da implementação.
- Memória não foi commitada no `main` (regra do repo proíbe commit direto); ver com o operador.
- Aceitação em Go: `internal/acceptance/` (binário real + `pending()` que reexecuta o teste num processo filho, marca `ACCEPT-FAIL:`/`SETUP-FAIL:`). Leva 60 a 76 s em todo `go test ./...`.
- O `zero-gate` NÃO vê achados de `unused`/`ineffassign` (o awk só conta linhas com código de regra tipo `QF1008:`): conferir o lint por conta própria, nunca confiar no "só o baseline" do PR (PR #36: `buildDir` sem uso passou com saída 3).
- Plataforma: o config do `zero` no Windows é `%AppData%\zero` e no macOS `XDG_CONFIG_HOME` ou `~/.config` (`internal/config/paths.go`). Harness que só grava em `XDG_CONFIG_HOME` quebra no Windows. O CI do fork (matriz com Windows e macOS) não roda, então ninguém vê.
- O operador já mesclou PR (#36) antes da revisão e da cyber-senior: revisar mesmo assim, veredito "corrigir (depois do merge)" com PR de correção.
