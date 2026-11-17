# Descrição do projeto: fork FabioLeitao/zero

Pedido do operador (Fabio), via Owl, em 2026-11-15 (brief `~/.cache/claude-pm/zero-pm-brief.md`). O "cliente" deste projeto é o operador.

## Contexto
- Clone: `~/Projects/dev/zero` (Go). `origin` = `FabioLeitao/zero` (fork). `upstream` = `Twigpine/zero` (terceiros, público, só leitura).
- Patches do fork ainda fora do upstream, por rótulo em `~/.config/zero-databoar/patch-labels.tsv`: `rmodel`, `sbxcompanion`, `acphard`. O script `~/.local/bin/zero-databoar-build` calcula a versão e falha (exit 6) se um rótulo já foi absorvido pelo upstream.
- Build instalado (T14 e Latitude): `0.13.0+up.9.ffd7bb21.databoar.0342e638.rmodel.sbxcompanion.acphard`.
- O `origin/main` do fork está em `eb2aea17` (v0.11.0+13, 24/out), atrasado em relação ao sync v0.13.0.

## Pedido
- **A.** Levar o sync `sync/upstream-v0.13.0-20261115` (merge `0342e638`, assinado) ao `main` do fork, por PR pronta e merge commit (nunca squash). O merge é do operador.
- **B.** Twigpine/zero#1148, `usage: record the provider-reported model on usage events when a router resolves an alias` (label `issue-approved`). Condições do mantenedor (comentário de 2026-10-16): tratar o valor como texto não confiável (remover controle e formato, limitar o tamanho, com teste de escape e de valor gigante); omitir o campo quando igual ao modelo pedido; 1ª PR só para chunks OpenAI-compatível e Codex `response.model`. PR ao upstream só com "vai" explícito do operador.
- Fluxo completo do PA (arquiteto, QA, dev, `cyber-senior`, revisor).

## Regras do operador
- Nada é escrito no upstream sem ordem explícita; todo `gh` de escrita leva `--repo FabioLeitao/zero` por extenso.
- `cyber-senior` em todo SHA entregue, antes do revisor (liberado pelo operador em 2026-11-16: "KP ok").
- Nunca PR em rascunho; um PR por épico; só abre pronto, com o gate local verde no SHA exato e o "vai" do operador. Commits assinados, sem trailer de ferramenta, sem squash, `reset --hard` ou force-push.

## Decisões do operador (2026-11-16)
1. Dev externo nas tarefas de código: Cursor na 2.1 (pequena e sensível), reserva OpenCode.
2. Gate local `zero-gate` (fora do repo, `umask 0022`), com a exceção conhecida do `tui`.
3. `TestRelativeAgeFormatsLastActivity` fica como exceção conhecida no plano; sem issue no upstream agora.
4. Tarefa A: PR pronta do sync, merge do operador. Tarefa B: desenho primeiro.

## Aprovação do plano
- 2026-11-16, ordem do operador via Owl: "(B) ... siga o fluxo do PA ate ficar pronta" e "(A) AGORA: abra UMA issue de bug" no upstream (feita: Twigpine/zero#1229). É a autorização para publicar as issues das tarefas 2.1 e 2.2 e seguir o fluxo da B.
- O operador ainda não leu o `docs/plano.md` por inteiro; a PR dos documentos (tarefa 1.2) é onde ele o aprova.
- Alteração das decisões (2026-11-16): o `TestRelativeAgeFormatsLastActivity` falha por fuso (não por data); o `zero-gate` roda com `TZ=UTC`; não há mais exceção conhecida de teste.
