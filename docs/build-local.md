# Build local do zero (upstream mais os patches do fork)

Um comando gera e instala o binário: o upstream na versão em que o `main` do fork foi sincronizado, mais os patches do fork. Tudo é determinístico (bash, git e go): nenhuma etapa usa modelo.

## Máquina nova, passo a passo

Pré-requisitos: `git`, `go` (a versão do `go.mod`), `bash`, `timeout` e acesso `ssh` ao GitHub.

```bash
mkdir -p ~/Projects/dev && cd ~/Projects/dev
git clone git@github.com:FabioLeitao/zero.git
cd zero
git remote add upstream https://github.com/Twigpine/zero.git   # só leitura; nunca se escreve nele
git fetch upstream --tags
mkdir -p ~/.local/bin
ln -sfn "$PWD/scripts/zero-databoar-build" ~/.local/bin/zero-databoar-build
zero-databoar-build          # UM comando: busca, calcula a versão, constrói, testa e instala
zero --version
```

## CI local no próprio build

Depois de construir e do smoke, o script roda, sem modelo e sem rede além do banco de vulnerabilidades:

- **Toolchain:** por padrão o Go do host. Em 18/nov o Go 1.27.2 do T14 gerava um binário com 2 vulnerabilidades da stdlib (`GO-2026-6859` em `net/http`, corrigida na 1.27.4, e `GO-2026-6809` em `crypto/x509`, na 1.27.3). `ZERO_DATABOAR_GOTOOLCHAIN=go1.27.4` constrói com outro toolchain (o Go o baixa para `~/go/pkg/mod`); com ele o `govulncheck` dá limpo. O toolchain `go1.26.11` do `go.mod` **não serve** para este worktree: ele falha no carimbo de VCS (`git status` roda fora do repo).
- **`govulncheck -mode binary`** no binário que será instalado: achado vira saída 7, com a dica do toolchain que corrige, e nada é instalado.
- **`gosec`** nos pacotes alterados em relação à base do upstream: só aviso (as ocorrências aparecem no log).

`ZERO_DATABOAR_SKIP_VULN=1` pula o `govulncheck`; não use para instalar. Cuidado com o disco: o toolchain baixado e os caches de build ficam em `~/go/pkg/mod` e `~/.cache/go-build` (vários GB), e o `/tmp` é tmpfs (RAM): o script não escreve binários lá.

O padrão é construir `origin/main`. Para validar outro branch sem instalar: `zero-databoar-build --ref <branch> --no-install`. O binário fica em `~/Projects/dev/zero-worktrees/databoar-build/zero`.

Variáveis: `ZERO_REPO` (o clone, padrão `~/Projects/dev/zero`), `ZERO_DATABOAR_WT`, `ZERO_DATABOAR_DEST` e `ZERO_DATABOAR_LABELS` (tabela de rótulos, padrão `build/patch-labels.tsv` do próprio repo, com `~/.config/zero-databoar/patch-labels.tsv` como reserva).

## Como ler a versão

`0.14.0+up.0.8d23195f.databoar.675da8c7.rmodel.sbxcompanion.acphard`

| Parte | Significa |
|---|---|
| `0.14.0` | a última tag do upstream contida no ref (o `merge-base` com o `upstream/main`) |
| `up.0.8d23195f` | commits do upstream depois da tag (0) e o commit-base |
| `databoar.675da8c7` | o commit do fork que foi construído (bate com `vcs.revision`) |
| `rmodel`, `sbxcompanion`, `acphard` | patches que ainda **só existem no fork**: o rótulo aparece enquanto os arquivos dele diferirem do upstream |

Um rótulo some sozinho quando o upstream absorve o patch. Ao construir o `main`, um rótulo que não difere mais do upstream **falha o build** (saída 6): é o aviso para tirá-lo de `build/patch-labels.tsv`.

## Os rótulos: `build/patch-labels.tsv`

Fonte de verdade, versionada no fork. Cada seção é um rótulo seguido dos caminhos que ele possui. A seção reservada `ignore:` lista arquivos de processo do fork (docs, testes de aceitação, o próprio script) que nunca viram rótulo.

Um arquivo que difere do upstream e não está em nenhuma seção **para o build** (saída 4). Para ver quais são, sem construir:

```bash
zero-databoar-build --check-labels   # lista os caminhos e sai 4; sai 0 quando tudo está coberto
```

Cada caminho listado vai para a seção do patch a que pertence, ou para `ignore:`. Todo arquivo novo de um patch entra no rótulo na mesma PR que o cria.

## Saídas do script

| Saída | Causa |
|---|---|
| 0 | construído (e instalado, sem `--no-install`) |
| 2 | opção desconhecida |
| 3 | ref ou tabela de rótulos ausente, ou falha de git |
| 4 | caminho que difere do upstream e não está em rótulo (`--check-labels` mostra qual) |
| 5 | o binário não bate com a versão ou com o commit |
| 6 | rótulo já absorvido pelo upstream, ao construir o `main` |
| 7 | `govulncheck` achou vulnerabilidade no binário (ou não está no `PATH`) |

## Rollback

A instalação guarda o binário anterior ao lado, com carimbo de data:

```bash
mv ~/.local/bin/zero.bak-<carimbo> ~/.local/bin/zero
mv ~/Projects/dev/zero/zero-linux-sandbox.bak-<carimbo> ~/Projects/dev/zero/zero-linux-sandbox
```

O script imprime esses dois comandos ao fim de cada instalação.

## Em outro host (latitude)

Instalar em host remoto é manual, com o `ssh` à vista: copiar o `zero` e o `zero-linux-sandbox`, conferir `sha256sum` nos dois lados antes de trocar, guardar `.bak-<carimbo>` e só então `mv`. Nunca trocar o binário com uma revisão do cyber-senior ou do KP rodando.

## Teste

`internal/buildlocal` roda o script contra repositórios de teste (sem rede, sem modelo, sem instalar) em todo `go test ./...`: confere a versão com rótulo, a saída 4 com `--check-labels` e a saída 6.
