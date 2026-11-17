# Plano do projeto: fork FabioLeitao/zero

Atualizado em 2026-11-16. O "cliente" e o operador (Fabio): não há cliente externo, por isso não há formulário de perguntas nem PDF de visão. A aprovação do plano é a do operador, no PR que traz este arquivo.

## Resumo
O `FabioLeitao/zero` é o fork do `Twigpine/zero` (Go) com patches próprios ainda fora do upstream (rótulos `rmodel`, `sbxcompanion` e `acphard` em `~/.config/zero-databoar/patch-labels.tsv`). Duas entregas:
1. **M1, fluxo e sync:** levar o `upstream/main` v0.13.0+9 para o `main` do fork por merge commit e montar o fluxo do PA neste repo.
2. **M2, #1148 (patch `rmodel`):** a issue `Twigpine/zero#1148`, aprovada pelo mantenedor com condições, vira um PR ao upstream.

**Regra nº 1:** nada é escrito no upstream sem o "vai" explícito do operador (ver `docs/operacao.md`). **Regra nº 2:** o `cyber-senior` roda a cada SHA entregue, antes do revisor.

## Fila de PRs (decidida antes de abrir a primeira)
| PR | Onde | Conteúdo | Quando abre |
|---|---|---|---|
| 1 | fork | tarefa 1.1, o sync `sync/upstream-v0.13.0-20261115` para o `main` | mesclada em 2026-11-17 (#28, `da0630da`) |
| 2 | fork | tarefa 1.2, `docs/operacao.md` e `docs/plano.md` | agora |
| 3 | fork | tarefa 2.1, etapa de desenho (R10 incluído) e testes de aceitação (arquiteto, depois QA, no mesmo branch) | depois da 1 e da 2 mescladas |
| 4 | fork | tarefa 2.1, implementação (dev externo), de um branch cortado do `upstream/main` | depois da 3 mesclada |
| 5 | upstream | tarefa 2.2, o mesmo branch da 4, só com o "vai" do operador | nunca sozinho |

Por que 3 e 4 separadas: o fluxo exige o desenho e os testes mesclados antes de liberar o dev. A 4 sai do `upstream/main`, contém só o escopo do #1148 e é reaproveitada na 5, para o PR do upstream nunca carregar os patches e os documentos do fork. A tarefa 1.3 não gera PR, porque o wrapper fica fora do repo.

## Requisitos
- **R1** O `main` do fork reflete o `upstream/main` v0.13.0+9 e mantém o histórico do upstream (merge commit, nunca squash).
- **R2** Os patches do fork (`rmodel`, `sbxcompanion`, `acphard`) seguem valendo depois do sync, e o `zero-databoar-build` fecha sem erro.
- **R3** Nada é escrito no upstream sem o "vai" do operador (PR, issue, comentário ou review).
- **R4** (#1148) O evento de uso registra o modelo que o provedor informou (`resolvedModel`), como atributo separado: `model` e preço ficam como estão.
- **R5** (#1148, condição do mantenedor) O valor é tratado como texto não confiável: remover caracteres de controle e de formato e limitar o tamanho (algumas centenas de bytes) antes de gravar, em todos os caminhos até o `zero usage report`, e também na leitura do relatório (o arquivo da sessão pode ter valor cru de versão anterior).
- **R6** (#1148) O campo é omitido quando igual ao modelo pedido, para `resolvedModel` significar "o roteador escolheu outro".
- **R7** (#1148) Escopo da 1ª PR: chunks OpenAI-compatível e Codex `response.model`. Outros provedores ficam para depois.
- **R10** (#1148, decisão do operador de 2026-11-16) `zero usage report` (texto e `--json`) mostra o `resolvedModel` só quando difere do modelo pedido; sem ele, nada aparece, sem erro e sem coluna vazia. O valor mostrado é o limpo e limitado (R5) também na leitura. Entra na 1ª PR da #1148 como **commit separado e descartável** (se o mantenedor pedir, sai sem afetar a gravação). Compatível com zero-bridge, zero-bridge-desktop e keen-platypus: leem o uso por chave, e a chave opcional é aditiva.
- **R8** Todo push passa no `zero-gate` (saída 0, ou 3 só pela exceção conhecida) no SHA exato, com `umask 0022`.
- **R9** Todo SHA entregue passa pelo `cyber-senior` antes do revisor.

## Tarefas
### M1: Fluxo e sync do fork (milestone "M1 Fluxo e sync")
- **1.1** *(gestão)* PR do sync para o `main` do fork. Branch `sync/upstream-v0.13.0-20261115`, merge `0342e638` (assinado), 33 commits sobre o `main` (`eb2aea17`, v0.11.0+13). Merge commit, `--repo FabioLeitao/zero`, merge do operador. Atende R1, R2, R8. Corpo do PR com: diff do `main`, o resultado do `zero-gate` e a exceção conhecida, o `go vet` e a versão do build (`0.13.0+up.9.ffd7bb21.databoar.0342e638.rmodel.sbxcompanion.acphard`). Depois do merge: aplicar a proteção do `main` (ver `docs/operacao.md`).
- **1.2** *(gestão)* `docs/operacao.md` e `docs/plano.md`, em PR do fork. Atende R3, R8, R9.
- **1.3** *(gestão, feita em 2026-11-16)* Wrapper `~/.local/bin/zero-gate` fora do repo e lista `~/.config/zero-gate/known-failures.txt`. Atende R8.

### M2: Upstream #1148, o patch `rmodel` (milestone "M2 Upstream 1148")
- **2.1** *(backend, `dev-externo`: Cursor)* `usage: record the provider-reported model on usage events when a router resolves an alias`. Reconferir a semântica do patch que já está no fork contra o `upstream/main` v0.13.0 (o upstream mexeu nos mesmos arquivos entre 0.12 e 0.13) e levá-lo ao que o mantenedor aprovou: R4 a R7, mais a exibição no relatório (R10, commit separado). Arquivos do rótulo `rmodel`: `internal/zeroruntime/{types,helpers}.go`, `internal/usage/report*.go`, `internal/providers/openai/*`. Desenho, testes de aceitação e implementação seguem o fluxo (PR 3, depois PR 4). Risco: **alto** (segurança, texto não confiável).
- **2.2** *(gestão)* PR ao `Twigpine/zero`. Só com o "vai" explícito do operador, de um branch do fork cortado do `upstream/main`, com `--repo Twigpine/zero` por extenso, `Fixes #1148`, seguindo o CONTRIBUTING do upstream: issue com `issue-approved` (já tem), foco só no #1148, descrição do que mudou e por quê, testes e verificação, sem trabalho fora do escopo, poucos itens abertos de uma vez. Atende R3.

**Dev externo da 2.1: Cursor, com justificativa por escrito.** O Cursor é eventual neste repo (só se crítico ou pedido pelo operador). Aqui o operador pediu: a tarefa é pequena e sensível, porque a falha seria deixar um valor controlado por um gateway chegar a um terminal; o operador tem franquia no Cursor. **Reserva: OpenCode**, se a franquia acabar. O `backend-dev-senior` do PA é Python + Docker, e o zero é Go.

## Testes (para o QA, caixa-preta, antes da implementação da 2.1)
Cada teste dá o resultado esperado e o requisito que cobre. Regra do AGENTS.md: o teste tem de falhar no código sem a mudança, pelo motivo declarado, e não pode ser barrado por uma guarda anterior; hermético (sem tocar a config, o cache ou o estado do desenvolvedor: redirecionar `HOME` e `XDG_*`).
- **T2.1** Chunk OpenAI-compatível com `model` diferente do pedido: o evento de uso traz `resolvedModel` com o valor. R4.
- **T2.2** Chunk com `model` igual ao pedido: o campo é omitido (evento idêntico ao de hoje). R6.
- **T2.3** Valor com sequência de escape (`\x1b[31m…`, OSC, NUL, C1 `\u0080-\u009f`) no stream: nenhum caractere de controle chega ao evento nem ao `zero usage report`. R5.
- **T2.4** Valor com caractere de formato (bidi U+202E, zero width, BOM): removido. R5.
- **T2.5** Valor gigante (muito acima do limite): gravado truncado no limite em bytes, sem partir um caractere UTF-8 ao meio. R5.
- **T2.6** Caso dividido: o controle no meio do valor (`gpt-\x00` + `4`) é removido **antes** de comparar com o modelo pedido. Se igual depois de limpar, o campo é omitido; o valor sem limpar passar não é evidência (regra "normalizar antes de casar"). R5, R6.
- **T2.7** Codex `response.model` diferente do pedido: `resolvedModel` presente, com a mesma limpeza. R4, R5, R7.
- **T2.8** Codex `response.model` igual ao pedido ou ausente: o campo é omitido. R6, R7.
- **T2.9** Provedor sem o campo no stream: nenhuma mudança no evento. R4.
- **T2.10** `model` e o cálculo de preço não mudam, com ou sem `resolvedModel`. R4.
- **T2.11** `zero usage report` (texto e `--json`) mostra o `resolvedModel` limpo, nunca o cru, só quando difere do pedido; sem ele, nenhuma coluna nem erro (saída 0); sessão com `events.jsonl` editado (valor cru) sai limpa; linhas mistas em ordem estável. R5, R10. Testes A2.1-11, A2.1-13 a A2.1-15.
- **T1.1** Depois do sync: `zero-databoar-build` fecha sem erro e a versão tem os rótulos que ainda diferem do upstream. R2.

## Exceções conhecidas do gate
- **Nenhuma no momento.** O `TestRelativeAgeFormatsLastActivity` (`internal/tui`) não depende da data: depende do fuso. Com `TZ=UTC` (o runner do CI) passa; com `America/Sao_Paulo` (-03) falha com `session_test.go:1331: last year: sessionAge = "2025-05-31", want "2025-06-01"`, no `upstream/main` `5942518f` limpo (passa em `Asia/Kolkata` e `Pacific/Auckland`). O `zero-gate` roda com `TZ=UTC`. Issue no upstream: **Twigpine/zero#1229**, aberta em 2026-11-16 com ordem explícita do operador; sem PR até o label `issue-approved`.
- **`lint-static`** (baseline adotado no `zero-gate`; o operador mesclou o #28 com saída 3): 5 achados no `upstream/main` `5942518f` (`installtest/workflow_permissions_test.go:20`, `proxydial/proxydial.go:67` e `:72`, `update/update.go:512`, `tools/web_fetch.go:330`) e mais 2 no `main` do fork, nos testes do `acphard` (`acp/agent_test.go:2825`, `acp/jsonrpc_test.go:1230`). O CI do upstream trata o `lint-static` como advisory.
- **Ambiente do T14 (já tratado no wrapper, sem afrouxar nada):** `$HOME` é repo git; o teste `TestBashToolRunsWithHostSandboxBackendWhenAvailable` deixa um `.git` no `TMPDIR` quando o helper de sandbox está no `PATH`; hook do operador em `~/.config/zero/hooks.json`.

## Versões
Não se aplica: sem builder, deployer ou documentador. Binário por `~/.local/bin/zero-databoar-build`.

## Perguntas
Nenhuma em aberto. Decididas pelo operador em 2026-11-16: dev externo (Cursor na 2.1, reserva OpenCode); o gate `zero-gate` com `umask 0022` e a exceção do `tui`; nada de issue no upstream agora; PR do sync liberada, merge do operador; 2.1 começa pelo desenho.
