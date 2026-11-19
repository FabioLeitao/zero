---
name: operar-zero
description: Delegar uma tarefa pequena e bem especificada ao próprio `zero` (agente de código em Go, provedor free-tier em modo auto) como dev externo, e operá-lo sem gastar cota do Claude Code. Use quando o operador autorizar o zero numa tarefa (linha `Dev externo:` do docs/operacao.md), para montar o brief, chamar o zero, retomar uma sessão que estourou os turnos, ler o uso e o modelo que o roteador escolheu, e usar os comandos de leitura dele (search, sessions, repo-map, context, doctor, sandbox).
---

# Operar o zero como dev externo

O zero é o agente de código do próprio projeto. Aqui ele faz o trivial, de graça (provedor `freellmapi-auto`, modelo `auto`, um roteador de free tiers). O plano, o desenho, as issues, o gate e a revisão ficam com o PM e os auditores. **Só delegue com a autorização do operador** e dentro da linha `Dev externo:` de `docs/operacao.md`; essa linha só o operador muda.

## Regras que não mudam
- O zero **não commita, não empurra e não abre PR**. Quem commita é o PM, assinado e sem trailer de ferramenta. Todo brief termina com "No git commit, no push, no network".
- O zero trabalha no **branch da tarefa**, no próprio clone (o operador não gosta de worktree destacado): `git switch -c <branch> origin/main`. Worktree só se o clone estiver sujo ou em uso por outra sessão, e removido ao fim da tarefa. Enquanto o zero edita, o PM não escreve no mesmo clone (commit só por caminho, `git commit -- <arquivo>`).
- Nada do que ele escreve entra sem o `zero-gate` no SHA exato e uma segunda opinião **só leitura** (Codex ou Hermes) antes de qualquer push.
- Limite de escopo no brief: arquivos permitidos, "não toque em X", e um teto de linhas (ex.: 60). Se estourar o teto ou tocar um contrato, pare e mostre ao operador.

## Pré-requisitos (aprendidos na prática)
- `zero exec` falha com saída 3 se a config listar uma raiz de escrita que não existe. Em 18/nov faltava `~/.cache/uv`: `mkdir -p ~/.cache/uv` resolve (alguém limpa o `~/.cache`; o `zero-gate` também perde os logs se isso acontecer no meio).
- **Go dentro da sandbox do zero.** `zero exec` é headless: nunca pede permissão, e a sandbox (`zero sandbox policy`: `mode=enforce`, `network=deny`, bwrap) nega rede. Pior: ela **troca `GOMODCACHE` e `GOCACHE`** por pastas privadas e vazias em `~/.cache/zero/runtime/v1/<hash>/`, então `go vet` e `go test` tentam baixar módulos e falham (`go: downloading ...`, `dial udp`, `module lookup disabled by GOPROXY=off`). Não é recusa do zero: ele tentou e a sandbox bloqueou. Medido em 18/nov: com o cache real indicado explicitamente, `go vet` passa dentro da sandbox. Passe sempre, no `exec` e no brief:
  ```bash
  zero exec|daemon run ... --add-dir ~/.cache/go-build ...     # só o cache de build precisa de escrita
  # e, no brief, todo comando go vai com este prefixo (o ambiente tem de ser dito no comando):
  GOMODCACHE=$HOME/go/pkg/mod GOCACHE=$HOME/.cache/go-build GOPROXY=off GOFLAGS=-mod=readonly go test ./<pacote>/ -count=1
  ```
  Medido em 19/nov dentro do daemon: só `--add-dir ~/.cache/go-build` sem `GOMODCACHE` explícito falha (`module lookup disabled by GOPROXY=off`); com o `GOMODCACHE` dito no comando passa, e a **leitura** de `~/go/pkg/mod` não precisa de `--add-dir`. Antes, rode `zero sandbox check bash --side-effect write --path ~/.cache/go-build` (decision=prompt é o esperado; o `--add-dir` libera). Nunca `--skip-permissions-unsafe` nem `--auto high`. O executor roda `go vet`/`go test` por pacote; `-race` e o gate ficam com o PM. Depois: `gosec -quiet ./<pacotes>/...` (offline) sem achado novo sobre a linha de base, e `govulncheck` só no PM, fora da sandbox.
- **Provedor e modelo.** O operador não tem API da Anthropic: **nunca `--mode` (os presets escolhem modelo Anthropic e falham com "belongs to anthropic, not openai-compatible") nem `--allow-escalation`**. Escolha o perfil por processo com `ZERO_PROVIDER=<perfil>` (não muda o provedor global) e o modelo com `-m`. Perfis de `zero providers list`: `freellmapi-auto` (free, modelo `auto`; em 19/nov exigiu retrabalho do PM nas duas tarefas), `chatgpt` (assinatura ChatGPT por OAuth: `gpt-5.6-luna` < `gpt-5.6-terra` < `gpt-6-luna`, do mais barato ao mais caro; sol não aparece) e perfis Ollama locais `zero-review-*` em `:11434/v1`. Comece pelo mais barato que sirva e registre o modelo no evento.
- **Daemon (sessão nomeada).** `zero daemon start` (o worker herda o ambiente do daemon: para usar `chatgpt`, suba o daemon com `ZERO_PROVIDER=chatgpt`), `zero daemon run --session <id> --cwd <pasta> --prompt "$(cat brief.md)" --add-dir ~/.cache/go-build --auto medium --max-turns 40 --tag <t> --session-title <t>`, `zero daemon status`, `zero daemon attach <id>` (o operador acompanha ao vivo) e `zero daemon stop` ao fim da rodada e a cada troca de binário. Saída em NDJSON (schemaVersion 2: reasoning/text/tool/usage/final/run_end). `--use-spec` rascunha a spec e para para revisão. `serve-remote` abre rede: só com ordem do operador.
- **Limite de turnos esgotado** (`exitCode 4`, "reached the max-turns limit"): a sessão deixa o trabalho pela metade e sem relatório. Confira o diff antes de aceitar (em 19/nov faltava uma linha de produção apagada, um dreno sem espera, dois testes errados e um arquivo lixo) e termine à mão, sem repetir a mesma chamada.
- `zero exec` só persiste a sessão com `--init-session-id <id>` (ou `-o stream-json`). Sem isso `zero usage report` fica vazio.

## Como chamar
**Padrão (skill `operar-dev-externo`):** o zero roda de forma persistente e visível, não em chamada avulsa. Duas formas: (1) `zero daemon run --session <tarefa> --cwd <pasta> --prompt "$(cat brief.md)" ...` (ver "Daemon" acima) e o operador acompanha com `zero daemon attach <tarefa>`; (2) a TUI interativa (`zero`) num painel do herdr: `zero` não é um `kind` do herdr, então `herdr pane run <id> zero`, `herdr pane send-text` e `herdr pane send-keys enter` (dois comandos, nunca num só), `herdr pane read`; confira o `pane_current_command` do painel antes de colar qualquer coisa. O `zero exec` abaixo é **exceção** (verificação pontual, sonda de leitura, repro): registre no evento da tarefa que foi `exec` e por quê, porque ele roda sem a sessão visível e sem aprovação na hora.

Exceção, `zero exec` avulso:
```bash
mkdir -p ~/.cache/uv
cd <worktree> && zero exec -C "$PWD" --auto medium --max-turns 40 \
  --init-session-id <tarefa>_<n> -f <brief>.md > <saida>.txt 2>&1
```
Rode em segundo plano e espere com um Monitor sobre o arquivo de saída; não faça polling. O exit 4 com "reached the max-turns limit" **não é falha de código**: o zero gastou os turnos lendo arquivos. Retome com `--resume <id>` e um brief curto e preciso:
```bash
zero exec -C "$PWD" --auto medium --max-turns 30 --resume <id> -f <brief2>.md
```
Dê no brief2 a saída exata dos testes que falharam e a causa provável. Isso converge em 2 a 3 rodadas; um brief que aponta o arquivo, a função e o resultado esperado gasta menos turnos que um aberto.

## Economizar turnos (a tarefa de 18/nov gastou 40+30+30 turnos relendo arquivos)
- Antes de chamar, cole no brief os trechos exatos que ele precisa (arquivo, função, linhas) ou rode `zero repo-map --query "<tema>"` e liste os 3 a 5 arquivos. Um brief que diz onde mexer poupa dezenas de leituras.
- O zero não tem `--specialist` no `exec`: ele roda o agente principal. Para só explorar, peça ao `explorer` (especialista só leitura, `zero specialist show explorer`); para editar, o `worker` é o que já age. Não usei nenhum nos 18/nov, nem slash-skill; as skills do zero (`golang-how-to`, `incremental-implementation`, `debugging-and-error-recovery`, `doubt-driven-development`) carregam por descrição. Nomeie no brief a que quer ("siga a skill incremental-implementation") e verifique se ele a usou (`zero search`).
- Entre rodadas, rode você mesmo `gofmt -l`, `go vet` e o teste do pacote e devolva só as falhas: o zero errou `t.Fatal` com formato e `var a, b int = -1, -1` (lint QF1011), coisas que uma checagem barata pega antes do gate.

## Um bom brief (formato)
1. Tarefa e achado, com arquivo e linha.
2. A abordagem **já decidida** ("implemente exatamente isto"), não uma pergunta aberta.
3. Testes primeiro (TDD): nomes e casos, e "veja falhar, depois implemente".
3b. **Invariantes que os testes devem provar**, uma linha cada, inclusive os que parecem óbvios. O bug que sobrou em 18/nov era um invariante não listado ("o modelo nomeado num chunk de conteúdo não se perde quando há dois usages"): o zero passou nos testes que escreveu e eu achei com um teste meu. Pergunte-se "que caso o implementador provavelmente esquece?" e escreva-o no brief.
4. Lista dos arquivos que pode tocar e dos que não pode.
5. Verificação que ele deve rodar e o que **não** rodar.
6. "Não faça mais nada. Sem commit, sem push, sem rede."

## Ler o resultado (determinístico, sem modelo)
- `zero usage report --session <id>` mostra requisições, tokens e, nas linhas `resolved models`, **qual modelo o roteador escolheu** (`auto -> poolside/laguna-xs-2.1`). A sessão retomada soma as rodadas.
- `git diff --stat` e `git diff -- <arquivos>` no worktree; confira o teto de linhas.
- `gofmt -l`, `go vet`, `go test -race` rodados pelo PM, e depois o gate.

## Flags do `zero exec` que ajudam o PM (conferidas no `zero exec --help`; **não testadas**, salvo o que vier dito)
- `--plan`: modo de planejamento só leitura, **esconde as ferramentas de escrita e de shell**. Para exploração e revisão de graça; não serve para quem precisa rodar teste.
- `--use-spec` (com `--spec-model`): redige uma especificação e **para para revisão** (`zero spec show|approve|reject`). **Armadilha medida em 19/nov (issue #47 do fork):** com hooks ou MCP sandboxados na config (os 6 hooks do ai-memory), o `protectedCreateMonitor` (`internal/sandbox/protected_create_linux.go`) apaga `.zero/` logo depois de a spec ser gravada, então `zero spec show` falha (`lstat ... no such file`) e `zero spec approve` não tem o que aprovar; a sessão do rascunho também não é retomável (`--resume` → "not resumable"). Com config mínima (`XDG_CONFIG_HOME=<tmp>`, um provedor, sem hooks nem MCP) o arquivo fica. Contornos: rascunhar com config mínima, ou extrair o texto do campo `plan` do `events.jsonl` da sessão e colá-lo no prompt de uma sessão nova (sem `--use-spec`), ou usar `--plan`.
- `--self-correct`: laço de verificar e corrigir depois da edição (a correção automática exige `--auto medium` ou `high`).
- `--mode smart|deep|fast|large|precise` escolhe o modelo por preset e `--exec-profile balanced|fast|thorough` o estilo do laço (turnos, esforço, escalonamento); flags explícitas vencem. Com o provedor `auto` ainda não medi o que cada preset muda.
- `--fork <id>` cria uma sessão nova a partir de outra; `-w/--worktree [nome]` e `--worktree-dir` fazem o próprio zero criar o worktree isolado (eu uso `git worktree add`, que é mais previsível).

## Comandos de leitura úteis (do `zero --help`)
- `zero search <texto> [--session-id id --type tipo --json]`: busca nos eventos persistidos de todas as sessões.
- `zero sessions list|tree|lineage|children <id>`: histórico e ramificação; `rewind-plan` e `compact-plan` mostram o que um rewind descartaria. `zero sessions prune --dry-run` lista o que a limpeza removeria e por que mantém o resto (nunca remove sessão aberta por outro processo); sem a flag usa `sessions.retentionDays` da config. Útil para o disco.
- `zero repo-map --query <texto>`: mapa determinístico do repo, arquivos ranqueados por consulta.
- `zero context`: orçamento de contexto do workspace.
- `zero doctor [--connectivity]`: saúde da config e do provedor.
- `zero sandbox policy|setup|check|grants`: política, preparo nativo, decisão para uma ação hipotética (`check <tool> --side-effect read|write|shell|network`) e concessões persistentes.
- `zero providers` (grupo: `list|current|catalog|add|check|use|models|...`), `zero models`, `zero specialist list|show|create|edit|path`, `zero skills list|info|add|remove`, `zero hooks list|add|remove|enable|disable`, `zero plugins list|info|add|remove`, `zero backends [doctor]`: o que está configurado e o diagnóstico sem conectar a MCP externo. Os números de `zero context` (ferramentas expostas e adiadas) variam com plugins e MCP.
- `zero context`: o que a sessão gasta de partida (em 18/nov: ~9,9 mil tokens, com prompt de sistema 3,4k, `AGENTS.md` 1,7k, mapa do workspace 0,9k e ferramentas 3,8k; 16 ferramentas expostas, 29 disponíveis, 14 adiadas). Com o modelo `auto` a janela de contexto aparece como **desconhecida**: o roteador não a informa, então o zero não sabe quando está perto do limite; mantenha o brief e os arquivos lidos curtos.
- `zero backends` e `zero plugins list`: servidores MCP (ai-memory, context7, exa, hermes-webui, memory, sequential-thinking), hooks (ai-memory em 6 eventos: pre/post tool, session start/end, specialist start/stop) e plugins de usuário (`kp-code-review`, ou seja, o Keen Platypus dentro do zero, e `zero-handoff`). Os hooks do ai-memory rodam em toda ferramenta: se um teste dependente de HOME falhar só na sua máquina, suspeite do hook (foi o caso de `TestNewHookDispatcherWithExtraFoldsPluginHooks`).
- O plugin `kp-code-review` ainda **não foi testado** como revisor dentro do zero; revisão de segurança continua pelo `cyber-senior`.
- `zero verify [--only ids]`: roda as verificações locais detectadas.
- `zero repo-info [--json]`: caracteriza um repo em segundos, sem modelo e sem rede (arquivos, linhas por linguagem, commits, branches, contribuidores e ritmo).
- `zero specialist list|show <nome>`: `explorer` (só leitura), `worker` (edita), `code-review` (só leitura) e os seus; `create` faz mais.

## Modos que existem e eu não testei (só o help)
- `zero daemon start|status|stop`, `zero daemon run --session <id> --prompt <texto>` e `zero daemon attach <id>`: sessão nomeada (o help só diz "Create/route a session and stream its output"): `attach` acompanha ao vivo, mas **uma sessão que já existe não aceita nova `run` com o mesmo nome** ("session already exists"), e um rascunho de spec não é retomável; para continuar, abra outra sessão com o contexto no prompt; `serve-remote` abre uma ponte TLS com token. Provavelmente melhor que o `exec` solto para tarefas longas; teste antes de depender.
- `zero serve --mcp`: o zero como servidor MCP por stdio (`--allow-unsafe-tools` expõe escrita e shell).
- `zero acp`: ACP por JSON-RPC em stdio, para editores; também dá eventos estruturados.
- `zero cron add "<5 campos>" --prompt ...`, `zero worktrees prepare|release` e `zero eval`: agendamento, worktree isolado e suítes de avaliação offline.
- `zero changes commit|push|pr`: **não use no fluxo do PM**; commit, push e PR seguem o ritual do repo.

## Skills, hooks e especialistas que o zero enxerga (`zero skills list`, `zero hooks list`, `zero specialist list`)
- **Duas raízes de skills.** `~/.local/share/zero/skills/` é do zero (as do Keen Platypus) e `~/.agents/skills/` é a pasta compartilhada com os outros harnesses (somente leitura para o zero). Duplicadas (`code-review`, `security-recon`) valem as do zero; o aviso `duplicate skill` é inofensivo.
- **As do KP são etapas de revisão faseada, não de edição:** `review-candidates`, `review-validate`, `review-refute`, `review-second-opinion` (cada uma "não chame ferramentas", julga só a evidência do prompt), mais `code-review`, `security-recon`, `deep-test` e `issue-triage`. É o que o `kp-review-phased` roda usando o zero como harness (por isso o `kp-doctor` confere o sufixo do binário). Não as use num brief de implementação.
- **Para implementar em Go, nomeie no brief:** `golang-how-to` (já carrega sozinha em código Go), `test-driven-development`, `incremental-implementation`, `debugging-and-error-recovery`, `source-driven-development` e `doubt-driven-development` (revisão adversarial da própria decisão). **`constraint-driven-development`** vigia o que o executor costuma fazer quando aperta: afrouxar um teste, pular, apagar assert. Na tarefa de 18/nov o zero mudou uma expectativa de teste para passar; vale pedir essa skill e conferir o diff dos `_test.go` linha a linha.
- **Hooks:** os 6 do `ai-memory` (`pre/post tool-use`, `session start/end`, `specialist start/stop`) rodam em tudo, com `--agent zero`: o zero registra o que faz na memória durável do operador.
- **Especialistas:** `explorer`, `worker` e `code-review` (builtin) e `code-review-coder` (do usuário, qwen2.5-coder-7b local, recebe o conteúdo inline porque não faz chamada de ferramenta).

## Quando não delegar
Decisão de contrato, de escopo ou de segurança; qualquer coisa que exija rodar `go test` na sandbox dele; trabalho que o operador não autorizou.

## Registro
Evento `coordenacao` na issue da tarefa e na `[registro]`: qual harness (zero e a versão), qual provedor e **qual modelo o roteador escolheu** (do `usage report`), quantas rodadas, o SHA entregue e o resultado do gate. Assine `— project-manager`.
