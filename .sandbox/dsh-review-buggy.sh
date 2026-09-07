#!/usr/bin/env bash
set -Eeuo pipefail
PROMPT_FILE="$(mktemp)"
trap 'rm -f "$PROMPT_FILE"' EXIT{
  cat << 'PROMPT_HEADER'
Você é um revisor de código. Leia o diff ou arquivos abaixo e encontre bugs reais.
--- CONTEÚDO PARA REVISAR ---
PROMPT_HEADER
  if [[ "${1:-}" == "--files" ]]; then
    shift
    for f in "$@"; do
      echo ""
      echo "=== ARQUIVO: $f ==="
      cat "$f"
    done
  else
    REF="${1:-@{upstream}...HEAD}"
    git diff "$REF" 2>&1 || git diff HEAD 2>&1
  fi
} > "$PROMPT_FILE"
echo "prompt montado ($(wc -c < "$PROMPT_FILE") bytes)" >&2
cat "$PROMPT_FILE" >&2
