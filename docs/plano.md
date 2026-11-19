# Plano do projeto: fork FabioLeitao/zero

Atualizado em 2026-11-18. O "cliente" e o operador (Fabio): não há cliente externo, por isso não há formulário de perguntas. A visão do projeto está em `docs/visao-projeto-2026-11-18.pdf`. A aprovação do plano é a do operador, no PR que traz este arquivo.

## Resumo
O `FabioLeitao/zero` é o fork do `Twigpine/zero` (Go) com patches próprios ainda fora do upstream (rótulos `rmodel`, `sbxcompanion` e `acphard` em `~/.config/zero-databoar/patch-labels.tsv`). Duas entregas:
1. **M1, fluxo e sync:** levar o `upstream/main` na hora do corte de cada sync (v0.13.0+9, depois a tag v0.14.0) para o `main` do fork por merge commit e montar o fluxo do PA neste repo.
2. **M2, #1148 (patch `rmodel`):** a issue `Twigpine/zero#1148`, aprovada pelo mantenedor com condições, vira um PR ao upstream.

**Regra nº 1:** nada é escrito no upstream sem o "vai" explícito do operador (ver `docs/operacao.md`). **Regra nº 2:** o `cyber-senior` roda a cada SHA entregue, antes do revisor.

## Estado em 2026-11-18
**Pronto:** syncs v0.13.0 e v0.14.0 no `main`; harness de aceitação corrigido (2.4); `zero-gate`; binário 0.14.0 instalado no T14 (1.5); 2.1 implementada e verificada; PR upstream #1234 aberta (a 2.2).

**Em andamento:** cyber-senior e revisor nas PRs #38/#39 (depois do boar-pm, dentro da cota do operador); checks e revisão do mantenedor na #1234.

**Pendências do operador (não são do fluxo):**
- 1.6, o latitude: só quando ele mandar (`ssh` visível, `sha256` dos dois lados).
- 1.7, o commit pós-tag #1220: decisão dele.
- CI do fork parado desde 2026-10-14, então a proteção do `main` não foi aplicada.
- Memórias dos agentes (revisor, arquiteto) não versionadas.
- `docs/custos.md` e este plano: entram por PR ou push ao `main`, ele decide (o push direto foi negado).
- Desfecho da #1229 e da #1148 no upstream.

**Achado do `kp-doctor` (2026-11-18, exit 1, 16 falhas):** só uma é do zero. O `harness-build` exige `hdrtimeout` e `acpusage` no sufixo da versão (`fpc/common/kpdoctor.pas:319`), mas esses dois patches entraram no upstream (#1106 e #1137) e saíram do sufixo no sync da v0.14.0, por desenho. É a lista do KP que está velha, não o binário. As outras 15 são do KP: 10 wrappers do PATH diferentes dos do repo, 4 tags do ollama ausentes e o `pipelock`. Por isso o pin do KP segue em 0.13.0. As suítes `bats` do KP que tocam o zero (`kp_build_stamp_version`, `kp_review_phased_acp`, `kp_review_phased_base`) passam (57 testes), mas usam stubs.

## Fila de PRs (decidida antes de abrir a primeira)
| PR | Onde | Conteúdo | Quando abre |
|---|---|---|---|
| 1 | fork | tarefa 1.1, o sync `sync/upstream-v0.13.0-20261115` para o `main` | mesclada em 2026-11-17 (#28, `da0630da`) |
| 2 | fork | tarefa 1.2, `docs/operacao.md` e `docs/plano.md` | mesclada |
| 2b | fork | tarefa 1.4, o sync `sync/upstream-v0.14.0-20261117` (tag `v0.14.0`) para o `main` | mesclada (#35) |
| 2c | fork | tarefa 2.4, correção do harness de aceitação | mesclada (#38, `f9079bcc`) |
| 3 | fork | tarefa 2.1, etapa de desenho (R10 incluído) e testes de aceitação (arquiteto, depois QA, no mesmo branch) | mesclada (#36) |
| 4 | fork | tarefa 2.1, implementação (dev externo), de um branch cortado do `upstream/main` | aberta no fork (#39, base `base/upstream-v0.14.0`), aguarda cyber-senior e revisor |
| 5 | upstream | tarefa 2.2, o mesmo branch da 4, só com o "vai" do operador | aberta pelo operador: `Twigpine/zero#1234` (head `f8f0b7d1`), aguarda checks e o mantenedor |

Por que 3 e 4 separadas: o fluxo exige o desenho e os testes mesclados antes de liberar o dev. A 4 sai do `upstream/main`, contém só o escopo do #1148 e é reaproveitada na 5, para o PR do upstream nunca carregar os patches e os documentos do fork. A tarefa 1.3 não gera PR, porque o wrapper fica fora do repo.

## Requisitos
- **R1** O `main` do fork reflete o `upstream/main` na hora do corte de cada sync (v0.13.0+9, depois a tag v0.14.0) e mantém o histórico do upstream (merge commit, nunca squash).
- **R2** Os patches do fork (`rmodel`, `sbxcompanion`, `acphard`) seguem valendo depois do sync, e o `zero-databoar-build` fecha sem erro.
- **R3** Nada é escrito no upstream sem o "vai" do operador (PR, issue, comentário ou review).
- **R4** (#1148) O evento de uso registra o modelo que o provedor informou (`resolvedModel`), como atributo separado: `model` e preço ficam como estão.
- **R5** (#1148, condição do mantenedor) O valor é tratado como texto não confiável: remover caracteres de controle e de formato e limitar o tamanho (algumas centenas de bytes) antes de gravar, em todos os caminhos até o `zero usage report`, e também na leitura do relatório (o arquivo da sessão pode ter valor cru de versão anterior).
- **R6** (#1148) O campo é omitido quando igual ao modelo pedido, para `resolvedModel` significar "o roteador escolheu outro".
- **R7** (#1148) Escopo da 1ª PR: chunks OpenAI-compatível e Codex `response.model`. Outros provedores ficam para depois.
- **R8** Todo push passa no `zero-gate` (saída 0, ou 3 só pela exceção conhecida) no SHA exato, com `umask 0022`.
- **R9** Todo SHA entregue passa pelo `cyber-senior` antes do revisor.
- **R10** (#1148, decisão do operador de 2026-11-16) `zero usage report` (texto e `--json`) mostra o `resolvedModel` só quando difere do modelo pedido; sem ele, nada aparece, sem erro e sem coluna vazia. O valor mostrado é o limpo e limitado (R5) também na leitura. Entra na 1ª PR da #1148 como **commit separado e descartável** (se o mantenedor pedir, sai sem afetar a gravação). Compatível com zero-bridge, zero-bridge-desktop e keen-platypus: leem o uso por chave, e a chave opcional é aditiva.
- **R11** O `zero usage report` (texto e `--json`) mostra o bloco de atividade sem argumento novo; os filtros atuais (`--session`, `--since`, `--days`) valem para ele.
- **R12** Contagens: sessões, prompts (mensagens do usuário), passos (chamadas ao modelo), tokens, resultados de ferramenta por status e taxa de sucesso, dias ativos e maior sequência. Os passos e os tokens batem com o `total` do relatório.
- **R13** Mapa de calor por dia da semana e mês, em caracteres de bloco, com reserva ASCII quando o locale não é UTF-8.
- **R14** Determinismo: o dia de cada evento e o "hoje" vêm do relógio injetado e do fuso dele; testes em UTC e em fuso fixo.
- **R15** Robustez: sem eventos, um dia só, evento sem data válida, payload inválido e sessão grande não derrubam o bloco nem o relatório; os limites que já existem no relatório ficam como estão e são declarados.
- **R16** Saída estável e legível: no máximo 80 colunas, nenhum texto lido de evento é impresso, Linux, macOS e Windows, nenhuma dependência nova.

## Tarefas
### M1: Fluxo e sync do fork (milestone "M1 Fluxo e sync")
- **1.1** [feita] *(gestão)* PR do sync para o `main` do fork. Branch `sync/upstream-v0.13.0-20261115`, merge `0342e638` (assinado), 33 commits sobre o `main` (`eb2aea17`, v0.11.0+13). Merge commit, `--repo FabioLeitao/zero`, merge do operador. Atende R1, R2, R8. Corpo do PR com: diff do `main`, o resultado do `zero-gate` e a exceção conhecida, o `go vet` e a versão do build (`0.13.0+up.9.ffd7bb21.databoar.0342e638.rmodel.sbxcompanion.acphard`). Depois do merge: aplicar a proteção do `main` (ver `docs/operacao.md`).
- **1.2** [feita] *(gestão)* `docs/operacao.md` e `docs/plano.md`, em PR do fork. Atende R3, R8, R9.
- **1.3** [feita] *(gestão, feita em 2026-11-16)* Wrapper `~/.local/bin/zero-gate` fora do repo e lista `~/.config/zero-gate/known-failures.txt`. Atende R8.
- **1.4** [feita] *(gestão, #29)* PR do sync da tag `v0.14.0` (merge `80db8d10`, 8 commits do upstream sobre o `ffd7bb21`) ao `main` do fork, pronta, merge commit, gate no SHA exato, corpo dizendo quais patches ficaram e quais saíram. Ficaram: `rmodel`, `acphard`, `sbxcompanion`. Saíram por já estarem no upstream: sandbox degradado (#1082), `ZERO_RESPONSE_HEADER_TIMEOUT` (#1106), eventos de uso no ACP (#1137) e o `.gitignore` do workspaceindex. O commit pós-tag #1220 fica fora de propósito (1.7). Atende R1, R2, R8.
- **1.5** [feita em 2026-11-18: `zero 0.14.0+up.0.8d23195f.databoar.f9079bcc.rmodel.sbxcompanion.acphard` no T14; rollback `~/.local/bin/zero.bak-20261118T084741`; o `kp-doctor` ainda aponta o sufixo sem `hdrtimeout` e `acpusage`, ver Estado] *(gestão, #30)* Versão do build derivada do git pelo `zero-databoar-build`, que agora compara com a base registrada no sync (o `merge-base` com o `upstream/main`), e instalação no T14 em `~/.local/bin/zero` com backup datado e rollback. Não trocar o binário com revisão do `cyber-senior` ou do KP rodando; o pin do KP fica em 0.13.0 até validar (`kp-doctor` e as suítes que dependem do `zero`). Depende da 1.4 mesclada.
- **1.6** [aguarda a ordem do operador] *(gestão, #31)* O mesmo no latitude, host remoto: `ssh` visível na sessão tmux `latitude`, `sha256` do binário conferido nos dois lados, aviso ao operador antes. Depende da 1.5.
- **1.7** *(gestão, #33, a avaliar depois)* Commit pós-tag `8288ae81` (#1220, OpenGateway default `auto`), deixado fora do sync da v0.14.0 de propósito: muda o default de um provedor e não entra sem decisão do operador.

### M2: Upstream #1148, o patch `rmodel` (milestone "M2 Upstream 1148")
- **2.1** [entregue: PR #39 do fork, `zero-gate` exit 3 em `440da802`; falta cyber-senior e revisor] *(backend, `dev-externo`: Cursor)* `usage: record the provider-reported model on usage events when a router resolves an alias`. Reconferir a semântica do patch que já está no fork contra o `upstream/main` v0.13.0 (o upstream mexeu nos mesmos arquivos entre 0.12 e 0.13) e levá-lo ao que o mantenedor aprovou: R4 a R7, mais a exibição no relatório (R10, commit separado). Arquivos do rótulo `rmodel`: `internal/zeroruntime/{types,helpers}.go`, `internal/usage/report*.go`, `internal/providers/openai/*`. Desenho, testes de aceitação e implementação seguem o fluxo (PR 3, depois PR 4). Risco: **alto** (segurança, texto não confiável).
- **2.2** [PR #1234 aberta pelo operador; acompanhar só leitura, sem push ao branch sem avisar] *(gestão)* PR ao `Twigpine/zero`. Só com o "vai" explícito do operador, de um branch do fork cortado do `upstream/main`, com `--repo Twigpine/zero` por extenso, `Fixes #1148`, seguindo o CONTRIBUTING do upstream: issue com `issue-approved` (já tem), foco só no #1148, descrição do que mudou e por quê, testes e verificação, sem trabalho fora do escopo, poucos itens abertos de uma vez. Atende R3.
- **2.3** *(gestão, #32)* Inventário dos patches sem trilha no upstream e rascunhos (um por item) em `docs/upstream/`, sem abrir nada. Nada novo no upstream enquanto a #1229 e a #1148 não andarem.
- **2.6** [#45, desenho pronto em `docs/desenho/2.6.md`] *(backend, só no fork)* Bloco de atividade no `zero usage report` (texto e `--json`, sem argumento novo): mapa de calor, sessões, prompts, passos, tokens, taxa de sucesso das ferramentas, dias ativos e maior sequência. Rótulo `ustats`. Atende R11 a R16. **Depende da PR #43** (tabela de rótulos no repo) entrar no `main`. Decisões do desenho aceitas pelo PM em 2026-11-18: mapa em grade de calendário, dia local pelo fuso do relógio injetado, contar todas as sessões e todo prompt de usuário, e o limite já existente do `BuildReport` (payload inválido de `provider_usage` derruba o relatório, do `rmodel`) fica como está e travado em teste. Executor: o próprio `zero` (skill `operar-zero`), plano B OpenCode (ordem do operador, 2026-11-18). Sequência: testes de aceitação A2.6-1..10 do QA, implementação no branch `feat/2.6-usage-activity` do clone, gate no SHA exato, segunda opinião só leitura (Codex ou Hermes), seção `ustats:` no `build/patch-labels.tsv`; commit local assinado, push e PR só com o "vai" do operador.
- **2.5** [#44, entregue: branch `usage/1148-resolved-model` empurrado pelo operador] *(backend, `dev-externo`: zero)* Achado `openai/provider.go:376` do CodeRabbit na `Twigpine/zero#1234`: um chunk só de modelo depois do último chunk de usage atualiza a atribuição sem criar outro registro de uso. Dev externo: o próprio zero (dogfooding, `freellmapi-auto`, modelo `auto`), só esta tarefa. Sem tocar `zeroruntime/types.go` nem o contrato `StreamEvent`; produção até ~60 linhas; `zero-gate` no SHA exato; segunda opinião só leitura (Codex ou Hermes) antes de o operador empurrar; commit no branch `usage/1148-resolved-model`.
- **2.4** [feita, #38] *(qa, #37)* Corrigir o harness de aceitação da 2.1 depois da revisão pós-merge do #36: variável não usada que quebrava o lint (B1), config gravado no lugar errado no Windows (B2) e o `pending()` que escondia falha sem marca (S1). Entra no `main` antes de o dev externo trazer `internal/acceptance/`.

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
- **`lint-static`** (baseline adotado no `zero-gate`; o operador mesclou o #28 com saída 3): 6 achados do staticcheck, 4 idênticos no `upstream/main` (`installtest/workflow_permissions_test.go`, `proxydial/proxydial.go` duas vezes, `tools/web_fetch.go`) e 2 do fork nos testes do `acphard` (`acp/agent_test.go`, `acp/jsonrpc_test.go`). O achado de `update/update.go` está coberto por `//nolint` e não aparece. O CI do upstream trata o `lint-static` como advisory.
- **Ambiente do T14 (já tratado no wrapper, sem afrouxar nada):** `$HOME` é repo git; o teste `TestBashToolRunsWithHostSandboxBackendWhenAvailable` deixa um `.git` no `TMPDIR` quando o helper de sandbox está no `PATH`; hook do operador em `~/.config/zero/hooks.json`.

## Versões
Não se aplica: sem builder, deployer ou documentador. Binário por `~/.local/bin/zero-databoar-build`.

## Perguntas
Nenhuma em aberto. Decididas pelo operador em 2026-11-16: dev externo (Cursor na 2.1, reserva OpenCode); o gate `zero-gate` com `umask 0022` e a exceção do `tui`; nada de issue no upstream agora; PR do sync liberada, merge do operador; 2.1 começa pelo desenho.
