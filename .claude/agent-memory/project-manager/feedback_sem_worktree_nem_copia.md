---
name: sem-worktree-nem-copia
description: Regra do operador (24/nov/2026, via pa-pm e Owl): nunca worktree, nunca cópia/clone de repo em /tmp ou em qualquer pasta; tudo em branch do clone canônico
metadata:
  type: feedback
---

**Regra:** NUNCA `git worktree add` (nem `zero -w/--worktree`, nem `zero worktrees`), NUNCA cópia ou clone do repositório em `/tmp`, `~/.cache` ou qualquer outra pasta, para nenhum agente nem dev externo. Todo trabalho vai num BRANCH do clone canônico (`git switch -c`); clone sujo se resolve com commit num branch. Temporário só para arquivo pequeno em `mktemp`, apagado por quem criou (`trap`), também em falha. Antes de apagar um temporário que já exista, guardar o trabalho no caminho canônico (commit e push) e conferir. Dizer isso em toda chamada a subagente e em todo brief de dev externo.
**Why:** o `/tmp` deste host é tmpfs (RAM); em 23/nov encheu o swap e quase travou a máquina duas vezes (as minhas duas cópias de 535 MB do hermes, mais outras). Palavras do operador: "USEM GIT DIREITO !!! BRANCH SERVE PRA ISSO !!!".
**How to apply:** o `scripts/zero-databoar-build` do fork constrói num worktree destacado (`~/Projects/dev/zero-worktrees/databoar-build`, 58 MB, criado por mim em 24/nov): CONFLITA com a regra, decisão do operador pendente (remover o worktree e mudar o script para construir no clone canônico num branch). Não apagar worktrees de outros: 22 dos 26 registrados são `locked` do próprio zero (`~/.local/state/zero/worktrees`), mais `fix-21-*` e fixtures em /tmp. O prompt fixo do `hermes-review-pr` manda `cp -a` em "scratch dir" sem dizer onde (avisei o Owl). O texto fixo do cyber-senior e do revisor-senior NÃO é alterado por mim: a regra deve estar nas definições dos agentes. Ver [[zero-rodada-2-10]].
