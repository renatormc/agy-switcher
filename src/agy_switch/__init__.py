"""agy-switch: switch between Antigravity CLI (agy) accounts."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

__version__ = "0.1.0"

# Files in the agy directory that belong to the logged-in account.
CREDENTIAL_FILES = ["antigravity-oauth-token"]
TOKEN_FILE = "antigravity-oauth-token"
NAME_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


class SwitchError(Exception):
    pass


# --------------------------------------------------------------------------- paths


def agy_home() -> Path:
    return Path(
        os.environ.get("AGY_HOME", Path.home() / ".gemini" / "antigravity-cli")
    ).expanduser()


def store_home() -> Path:
    return Path(
        os.environ.get("AGY_SWITCH_HOME", Path.home() / ".agy-switch")
    ).expanduser()


def profiles_dir() -> Path:
    return store_home() / "profiles"


def state_file() -> Path:
    return store_home() / "state.json"


def profile_dir(name: str) -> Path:
    return profiles_dir() / name


# --------------------------------------------------------------------------- helpers


def validate_name(name: str) -> str:
    if not NAME_RE.match(name):
        raise SwitchError(
            f"invalid profile name: {name!r} "
            "(use letters, digits, '.', '_' or '-')"
        )
    return name


def _ensure_private_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)
    os.chmod(path, 0o700)


def _atomic_copy(src: Path, dst: Path) -> None:
    """Atomically copy src to dst with 0600 permissions."""
    dst.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=dst.parent, prefix=f".{dst.name}.")
    try:
        with os.fdopen(fd, "wb") as out, open(src, "rb") as inp:
            shutil.copyfileobj(inp, out)
        os.chmod(tmp, 0o600)
        os.replace(tmp, dst)
    except BaseException:
        Path(tmp).unlink(missing_ok=True)
        raise


def _write_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.")
    with os.fdopen(fd, "w") as f:
        json.dump(data, f, indent=2)
        f.write("\n")
    os.chmod(tmp, 0o600)
    os.replace(tmp, path)


def _read_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text())
    except (FileNotFoundError, json.JSONDecodeError):
        return {}


def token_email(token_path: Path) -> str | None:
    return _read_json(token_path).get("id_token_email")


def load_state() -> dict:
    return _read_json(state_file())


def save_state(state: dict) -> None:
    _ensure_private_dir(store_home())
    _write_json(state_file(), state)


def active_profile() -> str | None:
    name = load_state().get("active")
    if name and profile_dir(name).is_dir():
        return name
    return None


def list_profiles() -> list[str]:
    if not profiles_dir().is_dir():
        return []
    return sorted(
        p.name for p in profiles_dir().iterdir()
        if p.is_dir() and (p / TOKEN_FILE).is_file()
    )


def profile_email(name: str) -> str | None:
    return token_email(profile_dir(name) / TOKEN_FILE)


def live_email() -> str | None:
    return token_email(agy_home() / TOKEN_FILE)


def _now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


# --------------------------------------------------------------------------- core


def snapshot_live(name: str) -> None:
    """Copy the current agy credentials into profile `name`."""
    src_dir = agy_home()
    if not (src_dir / TOKEN_FILE).is_file():
        raise SwitchError(
            f"no login found at {src_dir / TOKEN_FILE}. "
            "Log in to agy first (or use `agy-switch add NAME --login`)."
        )
    dst_dir = profile_dir(name)
    _ensure_private_dir(profiles_dir())
    _ensure_private_dir(dst_dir)
    for fname in CREDENTIAL_FILES:
        src = src_dir / fname
        if src.is_file():
            _atomic_copy(src, dst_dir / fname)
    meta = _read_json(dst_dir / "meta.json")
    meta.setdefault("created_at", _now())
    meta["updated_at"] = _now()
    meta["email"] = token_email(dst_dir / TOKEN_FILE)
    _write_json(dst_dir / "meta.json", meta)


def sync_active_back() -> None:
    """agy refreshes its token on its own; save the newest version back into the
    active profile (only if the live login is still the same account)."""
    active = active_profile()
    if not active:
        return
    live = live_email()
    if live and live == profile_email(active):
        snapshot_live(active)


def restore_profile(name: str) -> None:
    src_dir = profile_dir(name)
    dst_dir = agy_home()
    dst_dir.mkdir(parents=True, exist_ok=True)
    for fname in CREDENTIAL_FILES:
        src = src_dir / fname
        if src.is_file():
            _atomic_copy(src, dst_dir / fname)
        else:
            (dst_dir / fname).unlink(missing_ok=True)


def backup_live() -> Path | None:
    token = agy_home() / TOKEN_FILE
    if not token.is_file():
        return None
    bdir = store_home() / "backups"
    _ensure_private_dir(bdir)
    stamp = datetime.now().strftime("%Y%m%d-%H%M%S-%f")
    dst = bdir / f"{TOKEN_FILE}.{stamp}"
    _atomic_copy(token, dst)
    # keep only the 10 most recent backups
    old = sorted(bdir.glob(f"{TOKEN_FILE}.*"))[:-10]
    for p in old:
        p.unlink(missing_ok=True)
    return dst


# --------------------------------------------------------------------------- commands


def cmd_add(args: argparse.Namespace) -> int:
    name = validate_name(args.name)
    exists = (profile_dir(name) / TOKEN_FILE).is_file()
    if exists and not args.force:
        raise SwitchError(
            f"profile {name!r} already exists (use --force to overwrite)"
        )

    if args.login:
        sync_active_back()
        backup = backup_live()
        token = agy_home() / TOKEN_FILE
        token.unlink(missing_ok=True)
        print(
            "Opening agy to log in. Sign in with the desired account and, once "
            "logged in, exit agy (/exit or Ctrl+C)."
        )
        try:
            subprocess.run([args.agy_bin], check=False)
        except FileNotFoundError:
            if backup:
                _atomic_copy(backup, token)
            raise SwitchError(f"executable {args.agy_bin!r} not found")
        except KeyboardInterrupt:
            pass
        if not token.is_file():
            if backup:
                _atomic_copy(backup, token)
            raise SwitchError("login not completed; previous credentials restored")

    email = live_email()
    for other in list_profiles():
        if other != name and email and profile_email(other) == email:
            print(
                f"warning: account {email} is already saved in profile {other!r}",
                file=sys.stderr,
            )
    snapshot_live(name)
    state = load_state()
    state["active"] = name
    save_state(state)
    print(f"Profile {name!r} {'updated' if exists else 'added'} ({email or 'unknown email'}) and set as active.")
    return 0


def cmd_list(args: argparse.Namespace) -> int:
    names = list_profiles()
    if not names:
        print("No saved profiles. Use `agy-switch add NAME`.")
        return 0
    active = active_profile()
    width = max(len(n) for n in names)
    for n in names:
        mark = "*" if n == active else " "
        print(f"{mark} {n.ljust(width)}  {profile_email(n) or '-'}")
    return 0


def cmd_use(args: argparse.Namespace) -> int:
    name = validate_name(args.name)
    if name not in list_profiles():
        raise SwitchError(f"profile {name!r} does not exist (see `agy-switch list`)")
    if name == active_profile() and live_email() == profile_email(name):
        print(f"Already using {name!r} ({profile_email(name)}).")
        return 0
    sync_active_back()
    backup_live()
    restore_profile(name)
    state = load_state()
    state["active"] = name
    save_state(state)
    print(f"Now using {name!r} ({profile_email(name) or 'unknown email'}).")
    return 0


def cmd_current(args: argparse.Namespace) -> int:
    active = active_profile()
    live = live_email()
    if not active:
        if live:
            print(f"No active profile (agy is logged in as {live}, not saved).")
        else:
            print("No active profile.")
        return 1
    expected = profile_email(active)
    print(f"{active} ({expected or 'unknown email'})")
    if live != expected:
        print(
            f"warning: agy is logged in as {live or 'nobody'}, "
            f"which differs from profile {active!r}",
            file=sys.stderr,
        )
    return 0


def cmd_remove(args: argparse.Namespace) -> int:
    name = validate_name(args.name)
    pdir = profile_dir(name)
    if not pdir.is_dir():
        raise SwitchError(f"profile {name!r} does not exist")
    if not args.yes:
        answer = input(f"Remove profile {name!r} ({profile_email(name) or '-'})? [y/N] ")
        if answer.strip().lower() not in ("y", "yes"):
            print("Cancelled.")
            return 1
    shutil.rmtree(pdir)
    state = load_state()
    if state.get("active") == name:
        state["active"] = None
        save_state(state)
        print(
            f"Profile {name!r} removed. It was the active one; the current agy "
            "login was left unchanged."
        )
    else:
        print(f"Profile {name!r} removed.")
    return 0


# --------------------------------------------------------------------------- cli


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="agy-switch",
        description="Switch between Antigravity CLI (agy) accounts.",
    )
    p.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    sub = p.add_subparsers(dest="command", required=True, metavar="COMMAND")

    a = sub.add_parser("add", help="save the account currently logged in to agy as a profile")
    a.add_argument("name", metavar="NAME")
    a.add_argument(
        "--login", action="store_true",
        help="log out, open agy to sign in to another account, and save the result",
    )
    a.add_argument("-f", "--force", action="store_true", help="overwrite an existing profile")
    a.add_argument("--agy-bin", default=os.environ.get("AGY_BIN", "agy"), help=argparse.SUPPRESS)
    a.set_defaults(func=cmd_add)

    ls = sub.add_parser("list", aliases=["ls"], help="list saved profiles")
    ls.set_defaults(func=cmd_list)

    u = sub.add_parser("use", aliases=["switch"], help="switch to the given profile")
    u.add_argument("name", metavar="NAME")
    u.set_defaults(func=cmd_use)

    c = sub.add_parser("current", help="show the active profile")
    c.set_defaults(func=cmd_current)

    r = sub.add_parser("remove", aliases=["rm"], help="remove a saved profile")
    r.add_argument("name", metavar="NAME")
    r.add_argument("-y", "--yes", action="store_true", help="do not ask for confirmation")
    r.set_defaults(func=cmd_remove)
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return args.func(args)
    except SwitchError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
