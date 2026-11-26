---
name: feedback-escapes-invisiveis
description: Edit tool and Bash heredocs convert some \uXXXX escapes (U+200B, U+202E) into literal invisible chars in design docs; always re-grep and fix with chr()-based python
metadata:
  type: feedback
---

Ao escrever `\u200b`, `\u202e` e afins em desenhos (ex.: docs/desenho/2.1.md), o Edit e até um heredoc com aspas no Bash gravaram o caractere invisível literal (outros, como `\u001b` e `\u0085`, passaram como texto). Isso aconteceu de novo em 2026-11-17.

**Why:** o desenho da 2.1 já tinha sido corrompido assim antes; um caractere invisível no texto engana o dev e o QA, que copiam os escapes para os testes.

**How to apply:** depois de toda edição com escapes, rode com `LC_ALL=C.UTF-8` um `grep -cP` que inclua C0, C1 (`\x{80}-\x{9f}`), `\x{a0}`, `\x{ad}`, `\x{200B}-\x{200F}`, `\x{2028}-\x{202E}`, `\x{2060}-\x{206F}` e `\x{FEFF}`. Para corrigir, use um python que monte a classe com `chr(0x...)` e troque cada ocorrência por `chr(92)+'u%04x'`. Não escreva o escape literal no comando.
