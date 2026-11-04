# agy-switch

Alterna entre várias contas do **Antigravity CLI** (`agy`).

O login do `agy` fica em `~/.gemini/antigravity-cli/antigravity-oauth-token`.
O `agy-switch` guarda uma cópia desse arquivo para cada perfil, em `~/.agy-switch/profiles/<nome>/`
(permissões 0700/0600), e coloca de volta a cópia certa quando você troca de conta.

## Instalação

```bash
uv tool install .          # instala o comando `agy-switch` no PATH
# ou, para desenvolvimento:
uv run agy-switch --help
```

## Uso

```bash
# 1. Com o agy logado na conta pessoal:
agy-switch add personal

# 2. Adicionar outra conta (desloga, abre o agy para login e salva o resultado):
agy-switch add work --login

agy-switch list            # * marca o perfil ativo
agy-switch use personal
agy-switch use work
agy-switch current
agy-switch remove personal # pede confirmação (-y para pular)
```

Outras opções: `add --force` sobrescreve um perfil existente; `ls`, `switch` e `rm` são aliases.

## Detalhes

- O `agy` renova o token sozinho. Antes de trocar de conta, o `agy-switch` salva o token
  atual no perfil ativo, desde que o e-mail ainda seja o mesmo. Assim o perfil não fica
  com um token vencido.
- Antes de sobrescrever o login, o `agy-switch` guarda um backup em `~/.agy-switch/backups/` (mantém os 10 últimos).
- Feche as sessões abertas do `agy` antes de trocar de conta.
- Variáveis de ambiente: `AGY_HOME` (padrão `~/.gemini/antigravity-cli`) e
  `AGY_SWITCH_HOME` (padrão `~/.agy-switch`).
