# agy-switch

Switch between multiple **Antigravity CLI** (`agy`) accounts.

On Linux, `agy` stores its login in the system keyring (GNOME Keyring, KWallet or another
Secret Service provider). If no keyring is available, it uses
`~/.gemini/antigravity-cli/antigravity-oauth-token` instead. `agy-switch` keeps a copy of the
login for each profile in `~/.agy-switch/profiles/<name>/` and puts the right one back in
place when you switch accounts.

Saved profiles are unencrypted files, protected only by their permissions (0700/0600), even
when `agy` itself uses the keyring.

## Requirements

- Linux or macOS
- Python 3.12 or newer
- [pipx](https://pipx.pypa.io/)
- Antigravity CLI (`agy`) installed and on your `PATH`

## Installation

```bash
pipx install agy-switch
```

This installs the `agy-switch` command into `~/.local/bin`. If the shell can't find it,
add that directory to your `PATH`:

```bash
pipx ensurepath   # then open a new terminal
```

If your default Python is older than 3.12, point pipx to a newer one:

```bash
pipx install --python python3.12 agy-switch
```

Check the installation:

```bash
agy-switch --version
```

To update:

```bash
pipx upgrade agy-switch
```

To uninstall:

```bash
pipx uninstall agy-switch
```

Saved profiles in `~/.agy-switch` are not removed. Delete that directory if you don't need them.

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
- On macOS, only the file is supported. A login that `agy` keeps in the Keychain isn't detected.
- Environment variables: `AGY_HOME` (default `~/.gemini/antigravity-cli`) and
  `AGY_SWITCH_HOME` (default `~/.agy-switch`).

## Development

Run from a clone with [uv](https://docs.astral.sh/uv/):

```bash
uv run agy-switch --help
```

To publish a release, set the new version, then build and upload it:

```bash
uv version --bump patch   # or minor / major
rm -rf dist
uv build
uv publish                # needs a PyPI token in UV_PUBLISH_TOKEN
```
