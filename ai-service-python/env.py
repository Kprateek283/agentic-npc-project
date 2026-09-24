"""Environment readers shared by every module, so one convention holds for every variable:
values are stripped, numbers that fail to parse name their variable, and flags accept the
usual truthy spellings."""

import os

_TRUE = {"1", "true", "yes", "on"}


def env_str(name: str, default: str | None = None) -> str | None:
    value = os.getenv(name)
    return default if value is None else value.strip()


def _parse(name: str, default, cast, kind: str):
    raw = env_str(name)
    if raw is None:
        return default
    try:
        return cast(raw)
    except ValueError:
        raise ValueError(f"{name} must be {kind}, got {raw!r}") from None


def env_int(name: str, default: int) -> int:
    return _parse(name, default, int, "an integer")


def env_float(name: str, default: float) -> float:
    return _parse(name, default, float, "a number")


def env_bool(name: str, default: bool) -> bool:
    raw = env_str(name)
    return default if raw is None else raw.lower() in _TRUE
