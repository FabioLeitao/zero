---
name: sem-api-anthropic
description: O operador não tem nem terá API da Anthropic (só subscription do Claude Code, incompatível com o zero); nenhum harness externo pode depender de modelo Anthropic
metadata:
  type: feedback
---

O operador (Fabio) NÃO tem API key da Anthropic e nunca terá; usa só a subscription do Claude Code, que não serve ao `zero`. Todo provedor do zero, OpenCode, Pi etc. é openai-compatible (freellmapi-auto, openrouter, Ollama local, chatgpt oauth).

**Why:** em 19/nov, `zero daemon run --mode precise` falhou com "zero model claude-sonnet-4.5 belongs to anthropic, not openai-compatible": os presets `--mode` (smart, deep, fast, large, precise) escolhem modelos Anthropic. O operador cortou: "todos iriam recusar model da anthropic".

**How to apply:** nunca use `--mode` nem `--allow-escalation` nas chamadas ao zero e não recomende modelo Anthropic para executor ou revisor externo; escolha o perfil por `ZERO_PROVIDER=<perfil>` (por processo, sem `providers use`) e o modelo por `-m`. Claude fica só nos papéis pagos do time (QA, arquiteto, revisor, PM). Lembrar de corrigir a skill `operar-zero` e a lista de flags recomendadas ao Owl (já aprovada com `--mode precise`).

**Mas o operador tem GPT autenticado no zero (19/nov):** o perfil `chatgpt` (openai-compatible, login OAuth da assinatura ChatGPT, não API paga por token) serve `gpt-5.6-luna` (padrão do perfil), `gpt-5.6-terra` e `gpt-6-luna` (`zero providers models chatgpt`, medido em 19/nov; o registro local `zero models list --provider openai` só tem GPT-4.x e está velho). Uso: `ZERO_PROVIDER=chatgpt zero exec|daemon run ... -m gpt-5.6-luna`. Consome a cota da assinatura dele: usar onde o free-tier do freellmapi não rende (tarefas maiores como a 2.6) e registrar o modelo no evento.

**Ordem de custo dos GPT (operador, 19/nov):** a classe define o preço, do menor para o maior: luna < terra < sol; dentro da classe, número menor é mais barato (5.5 < 5.6 < 6.0). Disponíveis no perfil `chatgpt`: gpt-5.6-luna (o mais barato), gpt-5.6-terra, gpt-6-luna (sol não aparece). Regra: começar pelo mais barato que sirva (gpt-5.6-luna) e só subir se o trabalho falhar de forma que o modelo explique a falha; registrar o modelo usado no evento.

**Executor externo = TUI num split do herdr (ordem do operador, 19/nov):** "passar a usar os external dev no herdr split screen com tui para carregar tudo dos caras e não amarrar a mão deles". Fluxo: `herdr pane split --current --direction right --cwd "$PWD" --no-focus` -> `herdr pane run <id> zero` (zero não é `kind` do herdr) -> brief com `herdr pane send-text <id> "..."` e `herdr pane send-keys <id> enter` (dois comandos) -> `herdr pane read <id> --source recent-unwrapped --lines N`. Checar `pane_current_command` antes de colar em qualquer painel (incidente do Goose no author:2). `zero daemon run`/`exec` só como exceção registrada no evento. Painel do zero aberto em 19/nov: w8:p2 (modo "ask": cada ação pede aprovação do PM).
