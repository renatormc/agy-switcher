# agy-switch

Switch between multiple **Antigravity CLI** (`agy`) accounts.

`agy` stores its login in `~/.gemini/antigravity-cli/antigravity-oauth-token`.
`agy-switch` keeps a copy of that file for each profile in `~/.agy-switch/profiles/<name>/`
(permissions 0700/0600) and puts the right one back in place when you switch accounts.

## Requirements

- Linux or macOS
- [uv](https://docs.astral.sh/uv/) (it installs Python 3.12 automatically if needed)
- Antigravity CLI (`agy`) installed and on your `PATH`

Install uv if you don't have it:

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

## Installation

From the project directory:

```bash
git clone <repo-url> agy-switch   # or copy the project folder
cd agy-switch
uv tool install .
```

This installs the `agy-switch` command into `~/.local/bin`. If the shell can't find it,
add that directory to your `PATH`:

```bash
uv tool update-shell   # then open a new terminal
```

Check the installation:

```bash
agy-switch --version
```

To update after changing the code:

```bash
uv tool install . --reinstall
```

To uninstall:

```bash
uv tool uninstall agy-switch
```

Saved profiles in `~/.agy-switch` are not removed. Delete that directory if you don't need them.

### Running without installing

```bash
uv run agy-switch --help
```

## Usage

```bash
# 1. With agy logged in to your personal account:
agy-switch add personal

# 2. Add another account (logs out, opens agy so you can sign in, then saves it):
agy-switch add work --login

agy-switch list            # * marks the active profile
agy-switch use personal
agy-switch use work
agy-switch current
agy-switch remove personal # asks for confirmation (-y to skip)
```

| Command | Description |
| --- | --- |
| `add NAME [--login] [--force]` | Save the current agy login as a profile. `--login` signs in to a new account first. `--force` overwrites an existing profile. |
| `list` (`ls`) | List profiles and their emails. |
| `use NAME` (`switch`) | Switch agy to the given profile. |
| `current` | Show the active profile and warn if agy is logged in to a different account. |
| `remove NAME [-y]` (`rm`) | Delete a saved profile. |

## Details

- `agy` refreshes its token on its own. Before switching accounts, `agy-switch` saves the
  current token back into the active profile, as long as the email still matches. This
  keeps profiles from holding an expired token.
- Before overwriting the login, `agy-switch` saves a backup to `~/.agy-switch/backups/`
  (keeps the 10 most recent).
- Close any running `agy` sessions before switching accounts.
- Environment variables: `AGY_HOME` (default `~/.gemini/antigravity-cli`) and
  `AGY_SWITCH_HOME` (default `~/.agy-switch`).
