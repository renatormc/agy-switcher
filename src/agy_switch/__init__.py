"""agy-switch: alterna entre contas do Antigravity CLI (agy)."""

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

# Arquivos do diretório do agy que pertencem à conta logada.
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
            f"nome de perfil inválido: {name!r} "
            "(use letras, números, '.', '_' ou '-')"
        )
    return name


def _ensure_private_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)
    os.chmod(path, 0o700)


def _atomic_copy(src: Path, dst: Path) -> None:
    """Copia src para dst de forma atômica e com permissão 0600."""
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
    """Copia as credenciais atuais do agy para o perfil `name`."""
    src_dir = agy_home()
    if not (src_dir / TOKEN_FILE).is_file():
        raise SwitchError(
            f"nenhum login encontrado em {src_dir / TOKEN_FILE}. "
            "Faça login no agy primeiro (ou use `agy-switch add NOME --login`)."
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
    """O agy renova o token sozinho; salva a versão mais nova no perfil ativo
    (apenas se o login atual ainda for da mesma conta)."""
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
    # mantém só os 10 backups mais recentes
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
            f"o perfil {name!r} já existe (use --force para sobrescrever)"
        )

    if args.login:
        sync_active_back()
        backup = backup_live()
        token = agy_home() / TOKEN_FILE
        token.unlink(missing_ok=True)
        print(
            "Abrindo o agy para login. Entre com a conta desejada e, depois de "
            "logado, saia do agy (/exit ou Ctrl+C)."
        )
        try:
            subprocess.run([args.agy_bin], check=False)
        except FileNotFoundError:
            if backup:
                _atomic_copy(backup, token)
            raise SwitchError(f"executável {args.agy_bin!r} não encontrado")
        except KeyboardInterrupt:
            pass
        if not token.is_file():
            if backup:
                _atomic_copy(backup, token)
            raise SwitchError("login não concluído; credenciais anteriores restauradas")

    email = live_email()
    for other in list_profiles():
        if other != name and email and profile_email(other) == email:
            print(
                f"aviso: a conta {email} já está salva no perfil {other!r}",
                file=sys.stderr,
            )
    snapshot_live(name)
    state = load_state()
    state["active"] = name
    save_state(state)
    print(f"Perfil {name!r} {'atualizado' if exists else 'adicionado'} ({email or 'e-mail desconhecido'}) e marcado como ativo.")
    return 0


def cmd_list(args: argparse.Namespace) -> int:
    names = list_profiles()
    if not names:
        print("Nenhum perfil salvo. Use `agy-switch add NOME`.")
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
        raise SwitchError(f"perfil {name!r} não existe (veja `agy-switch list`)")
    if name == active_profile() and live_email() == profile_email(name):
        print(f"Já usando {name!r} ({profile_email(name)}).")
        return 0
    sync_active_back()
    backup_live()
    restore_profile(name)
    state = load_state()
    state["active"] = name
    save_state(state)
    print(f"Agora usando {name!r} ({profile_email(name) or 'e-mail desconhecido'}).")
    return 0


def cmd_current(args: argparse.Namespace) -> int:
    active = active_profile()
    live = live_email()
    if not active:
        if live:
            print(f"Nenhum perfil ativo (agy logado como {live}, não salvo).")
        else:
            print("Nenhum perfil ativo.")
        return 1
    expected = profile_email(active)
    print(f"{active} ({expected or 'e-mail desconhecido'})")
    if live != expected:
        print(
            f"aviso: o agy está logado como {live or 'ninguém'}, "
            f"diferente do perfil {active!r}",
            file=sys.stderr,
        )
    return 0


def cmd_remove(args: argparse.Namespace) -> int:
    name = validate_name(args.name)
    pdir = profile_dir(name)
    if not pdir.is_dir():
        raise SwitchError(f"perfil {name!r} não existe")
    if not args.yes:
        answer = input(f"Remover o perfil {name!r} ({profile_email(name) or '-'})? [s/N] ")
        if answer.strip().lower() not in ("s", "sim", "y", "yes"):
            print("Cancelado.")
            return 1
    shutil.rmtree(pdir)
    state = load_state()
    if state.get("active") == name:
        state["active"] = None
        save_state(state)
        print(
            f"Perfil {name!r} removido. Ele era o ativo; o login atual do agy "
            "não foi alterado."
        )
    else:
        print(f"Perfil {name!r} removido.")
    return 0


# --------------------------------------------------------------------------- cli


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="agy-switch",
        description="Alterna entre contas do Antigravity CLI (agy).",
    )
    p.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    sub = p.add_subparsers(dest="command", required=True, metavar="COMANDO")

    a = sub.add_parser("add", help="salva a conta logada no agy como um perfil")
    a.add_argument("name", metavar="NOME")
    a.add_argument(
        "--login", action="store_true",
        help="desloga, abre o agy para fazer login em outra conta e salva o resultado",
    )
    a.add_argument("-f", "--force", action="store_true", help="sobrescreve perfil existente")
    a.add_argument("--agy-bin", default=os.environ.get("AGY_BIN", "agy"), help=argparse.SUPPRESS)
    a.set_defaults(func=cmd_add)

    ls = sub.add_parser("list", aliases=["ls"], help="lista os perfis salvos")
    ls.set_defaults(func=cmd_list)

    u = sub.add_parser("use", aliases=["switch"], help="troca para o perfil informado")
    u.add_argument("name", metavar="NOME")
    u.set_defaults(func=cmd_use)

    c = sub.add_parser("current", help="mostra o perfil ativo")
    c.set_defaults(func=cmd_current)

    r = sub.add_parser("remove", aliases=["rm"], help="remove um perfil salvo")
    r.add_argument("name", metavar="NOME")
    r.add_argument("-y", "--yes", action="store_true", help="não pede confirmação")
    r.set_defaults(func=cmd_remove)
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return args.func(args)
    except SwitchError as e:
        print(f"erro: {e}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
