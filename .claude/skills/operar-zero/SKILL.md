---
name: operar-zero
description: Delegar uma tarefa pequena e bem especificada ao próprio `zero` (agente de código em Go, provedor free-tier em modo auto) como dev externo, e operá-lo sem gastar cota do Claude Code. Use quando o operador autorizar o zero numa tarefa (linha `Dev externo:` do docs/operacao.md), para montar o brief, chamar o zero, retomar uma sessão que estourou os turnos, ler o uso e o modelo que o roteador escolheu, e usar os comandos de leitura dele (search, sessions, repo-map, context, doctor, sandbox).
---

# Operar o zero como dev externo

O zero é o agente de código do próprio projeto. Aqui ele faz o trivial, de graça (provedor `freellmapi-auto`, modelo `auto`, um roteador de free tiers). O plano, o desenho, as issues, o gate e a revisão ficam com o PM e os auditores. **Só delegue com a autorização do operador** e dentro da linha `Dev externo:` de `docs/operacao.md`; essa linha só o operador muda.

## Regras que não mudam
- O zero **não commita, não empurra e não abre PR**. Quem commita é o PM, assinado e sem trailer de ferramenta. Todo brief termina com "No git commit, no push, no network".
- O zero trabalha num **worktree** do branch da tarefa, nunca no clone canônico: `git worktree add -B <branch> <pasta> origin/<branch>`.
- Nada do que ele escreve entra sem o `zero-gate` no SHA exato e uma segunda opinião **só leitura** (Codex ou Hermes) antes de qualquer push.
- Limite de escopo no brief: arquivos permitidos, "não toque em X", e um teto de linhas (ex.: 60). Se estourar o teto ou tocar um contrato, pare e mostre ao operador.

## Pré-requisitos (aprendidos na prática)
- `zero exec` falha com saída 3 se a config listar uma raiz de escrita que não existe. Em 18/nov faltava `~/.cache/uv`: `mkdir -p ~/.cache/uv` resolve (alguém limpa o `~/.cache`; o `zero-gate` também perde os logs se isso acontecer no meio).
- A sandbox do zero **não baixa módulos Go** (`go test` falha com `reading go.mod` / `GOPROXY=off`). Diga no brief: "não rode go test"; o PM roda `go vet` e `go test` e devolve a saída.
- `zero exec` só persiste a sessão com `--init-session-id <id>` (ou `-o stream-json`). Sem isso `zero usage report` fica vazio.

## Como chamar
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

## Um bom brief (formato)
1. Tarefa e achado, com arquivo e linha.
2. A abordagem **já decidida** ("implemente exatamente isto"), não uma pergunta aberta.
3. Testes primeiro (TDD): nomes e casos, e "veja falhar, depois implemente".
4. Lista dos arquivos que pode tocar e dos que não pode.
5. Verificação que ele deve rodar e o que **não** rodar.
6. "Não faça mais nada. Sem commit, sem push, sem rede."

## Ler o resultado (determinístico, sem modelo)
- `zero usage report --session <id>` mostra requisições, tokens e, nas linhas `resolved models`, **qual modelo o roteador escolheu** (`auto -> poolside/laguna-xs-2.1`). A sessão retomada soma as rodadas.
- `git diff --stat` e `git diff -- <arquivos>` no worktree; confira o teto de linhas.
- `gofmt -l`, `go vet`, `go test -race` rodados pelo PM, e depois o gate.

## Comandos de leitura úteis (do `zero --help`)
- `zero search <texto> [--session-id id --type tipo --json]`: busca nos eventos persistidos de todas as sessões.
- `zero sessions list|tree|lineage|children <id>`: histórico e ramificação; `rewind-plan` e `compact-plan` mostram o que um rewind descartaria.
- `zero repo-map --query <texto>`: mapa determinístico do repo, arquivos ranqueados por consulta.
- `zero context`: orçamento de contexto do workspace.
- `zero doctor [--connectivity]`: saúde da config e do provedor.
- `zero sandbox policy|check|grants`: política e concessões de sandbox.
- `zero providers`, `zero models`, `zero specialist list`, `zero skills list`: o que está configurado.
- `zero verify [--only ids]`: roda as verificações locais detectadas.

## Modos que existem e eu não testei (só o help)
- `zero daemon start|status|stop`, `zero daemon run --session <id> --prompt <texto>` e `zero daemon attach <id>`: sessão nomeada que se acompanha e retoma; `serve-remote` abre uma ponte TLS com token. Provavelmente melhor que o `exec` solto para tarefas longas; teste antes de depender.
- `zero serve --mcp`: o zero como servidor MCP por stdio (`--allow-unsafe-tools` expõe escrita e shell).
- `zero acp`: ACP por JSON-RPC em stdio, para editores; também dá eventos estruturados.
- `zero cron add "<5 campos>" --prompt ...`, `zero worktrees prepare|release` e `zero eval`: agendamento, worktree isolado e suítes de avaliação offline.
- `zero changes commit|push|pr`: **não use no fluxo do PM**; commit, push e PR seguem o ritual do repo.

## Quando não delegar
Decisão de contrato, de escopo ou de segurança; qualquer coisa que exija rodar `go test` na sandbox dele; trabalho que o operador não autorizou.

## Registro
Evento `coordenacao` na issue da tarefa e na `[registro]`: qual harness (zero e a versão), qual provedor e **qual modelo o roteador escolheu** (do `usage report`), quantas rodadas, o SHA entregue e o resultado do gate. Assine `— project-manager`.
