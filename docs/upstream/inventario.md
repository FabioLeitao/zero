# Inventário dos patches do fork sem trilha no upstream

Conferido em 2026-11-17 contra a tag `v0.14.0` e o `upstream/main` (leitura, nada enviado ao `Twigpine/zero`). Regra: nada é aberto no upstream sem o "vai" do operador, e nada novo enquanto a #1229 e a #1148 não andarem.

| Patch (rótulo) | O que faz | Estado no upstream | Destino |
|---|---|---|---|
| `rmodel` | Grava no evento de uso o modelo que o provedor informou (`resolvedModel`), chunks OpenAI-compatível e Codex | #1148, `issue-approved` com condição de sanitizar; sem PR | Tarefa 2.1 (implementação) e 2.2 (PR, só com "vai") |
| `acphard` | Limites de tamanho de frame e de goroutines no ACP (cópia rebaseada da PR #944) | Issue #923 aprovada (de terceiro); PR #944 de terceiro, aberta, sem label | Não abrir PR nova. Rascunho `acphard.md` |
| `sbxcompanion` | `make build` também compila o `zero-linux-sandbox` no Linux; `make clean` o remove | Não coberto: o README o trata como passo manual opcional | Melhoria: Discussions > Ideas. Rascunho `sbxcompanion.md` |
| ~~sandbox degradado~~ | Avisar quando o sandbox da sessão está degradado | Saiu no sync da v0.14.0: o upstream mesclou a #1082; a nossa #1107 foi fechada | Nenhum |
| ~~`ZERO_RESPONSE_HEADER_TIMEOUT`~~ | Timeout opt-in do primeiro byte | Mesclado (#1106) | Nenhum |
| ~~eventos de uso no ACP~~ | Persistir uso de tokens de sessões ACP | Mesclado (#1137) | Nenhum |
| ~~workspaceindex respeita `.gitignore`~~ | Índice do workspace ignora o que o `.gitignore` ignora | Já está no upstream | Nenhum |

Abertas por nós e ainda sem aprovação: issue #1229 (teste de fuso) e as Discussions #1119, #1141 e #1151. Nenhuma delas vira PR antes do `issue-approved`.

Fora do sync de propósito (a avaliar depois): `8288ae81`, `feat(providers): default Twigpine OpenGateway to the auto smart-routing model (#1220)`, o único commit do `upstream/main` depois da tag `v0.14.0`. Muda o default de um provedor para o modelo `auto`; não entra sem decisão do operador.
