# agy-switch

Switch between multiple **Antigravity CLI** (`agy`) accounts.

On Linux, `agy` stores its login in the system keyring (GNOME Keyring, KWallet or another
Secret Service provider). If no keyring is available, it uses
`~/.gemini/antigravity-cli/antigravity-oauth-token` instead. `agy-switch` keeps a copy of the
login for each profile in `~/.agy-switch/profiles/<name>/` and puts the right one back in
place when you switch accounts.

On Windows, `agy` stores its login in the Windows Credential Manager (a generic credential
named `gemini:antigravity`), and `agy-switch` reads and writes it there.

Saved profiles are unencrypted files, protected only by their permissions (0700/0600), even
when `agy` itself uses the keyring.

## Requirements

- Linux, macOS or Windows (amd64 or arm64)
- Antigravity CLI (`agy`) installed and on your `PATH`

## Installation

`agy-switch` is a single self-contained binary. Copy and paste one of these commands.

**Linux / macOS** (installs to `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/renatormc/agy-switcher/master/install.sh | sh
```

**Windows** (PowerShell; installs to `%LOCALAPPDATA%\Programs\agy-switch` and adds it to your user `PATH`):

```powershell
irm https://raw.githubusercontent.com/renatormc/agy-switcher/master/install.ps1 | iex
```

Open a new terminal afterwards if the command isn't found, then check the installation:

```bash
agy-switch --version
```

To update, run the install command again. To install a specific version, set `VERSION`
(Linux/macOS) or `AGY_SWITCH_VERSION` (Windows) to a tag such as `v0.3.0` first. The install
directory can be changed with `INSTALL_DIR` / `AGY_SWITCH_INSTALL_DIR`.

You can also download a binary from the
[releases page](https://github.com/renatormc/agy-switcher/releases), or build it with Go:

```bash
go install github.com/renatormc/agy-switcher/cmd/agy-switch@latest
```

To uninstall, delete the `agy-switch` binary. Saved profiles in `~/.agy-switch` are not
removed. Delete that directory if you don't need them.

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
- On Windows, `~` means your user folder (`%USERPROFILE%`), so profiles live in
  `%USERPROFILE%\.agy-switch`. File permissions are not enforced there; the profiles are
  protected only by your user account's folder permissions.
- Environment variables: `AGY_HOME` (default `~/.gemini/antigravity-cli`) and
  `AGY_SWITCH_HOME` (default `~/.agy-switch`).

## Development

Requires [Go](https://go.dev/dl/) 1.27 or newer.

```bash
go run ./cmd/agy-switch --help
go test ./...
go build -o agy-switch ./cmd/agy-switch
```

To publish a release, push a version tag. The
[Release workflow](.github/workflows/release.yml) runs the tests, builds binaries for
Linux, macOS and Windows (amd64 and arm64) and attaches them to a new GitHub release:

```bash
git tag v0.3.0
git push origin v0.3.0
```
