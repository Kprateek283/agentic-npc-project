"""Shared helpers for the benchmark scripts: timing stats and result files."""

import json
import platform
import statistics
import subprocess
from datetime import date
from pathlib import Path

RESULTS_DIR = Path(__file__).parent / "results"


def percentile(values, pct):
    """Nearest-rank percentile. Small samples make interpolation false precision."""
    if not values:
        return None
    ordered = sorted(values)
    k = max(0, min(len(ordered) - 1, int(round(pct / 100.0 * len(ordered) + 0.5)) - 1))
    return ordered[k]


def stats(samples_ms):
    """Summarise a list of millisecond timings."""
    if not samples_ms:
        return {"n": 0}
    return {
        "n": len(samples_ms),
        "median_ms": round(statistics.median(samples_ms), 2),
        "p95_ms": round(percentile(samples_ms, 95), 2),
        "min_ms": round(min(samples_ms), 2),
        "max_ms": round(max(samples_ms), 2),
        "mean_ms": round(statistics.fmean(samples_ms), 2),
    }


def hardware() -> dict:
    """Machine facts, recorded alongside every result — timings mean nothing without them."""
    info = {"platform": platform.platform(), "python": platform.python_version()}
    try:
        cpu = subprocess.run(["lscpu"], capture_output=True, text=True, timeout=10).stdout
        for line in cpu.splitlines():
            if line.startswith("Model name:"):
                info["cpu"] = line.split(":", 1)[1].strip()
            elif line.startswith("CPU(s):") and "cpus" not in info:
                info["cpus"] = line.split(":", 1)[1].strip()
    except Exception:
        pass
    try:
        mem = Path("/proc/meminfo").read_text().splitlines()[0]
        info["memory"] = mem.split(":", 1)[1].strip()
    except Exception:
        pass
    try:
        gpu = subprocess.run(["nvidia-smi", "--query-gpu=name", "--format=csv,noheader"],
                             capture_output=True, text=True, timeout=10)
        info["gpu"] = gpu.stdout.strip() or "none detected"
    except Exception:
        info["gpu"] = "none detected"
    return info


def write_result(name: str, payload: dict) -> Path:
    RESULTS_DIR.mkdir(exist_ok=True)
    payload = {"benchmark": name, "run_date": date.today().isoformat(),
               "hardware": hardware(), **payload}
    path = RESULTS_DIR / f"{name}.json"
    path.write_text(json.dumps(payload, indent=2) + "\n")
    print(f"\nresults -> {path}")
    return path
