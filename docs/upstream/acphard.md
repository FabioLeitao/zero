# acphard (rascunho, NÃO abrir)

Situação: o upstream já tem a issue #923 (`issue-approved`, de terceiro) e a PR #944 (de terceiro, aberta, sem label, última atualização em 2026-11-03). O fork carrega uma cópia rebaseada dessa PR. Abrir outra PR duplicaria a #944.

Se o operador mandar agir, a única ação cabível é um comentário curto na #944 (ou na #923), em inglês, com evidência e sem despejo:

> We maintain a downstream fork that carries a rebased copy of this change, together with its test suite. If it helps the review, we can share the list of tests it adds (oversized frame, saturated requests, busy-reply bursts, stalled handlers).

Antes de qualquer envio: reler o estado da #944 (a branch dela pode ter mudado), conferir se o rebase do fork ainda bate com ela e pedir o "vai" do operador. Não enviar nada do fork como PR própria.
