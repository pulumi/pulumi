"""
Version-string helpers shared by the orchestrator's activities and the CLI
scripts (`scripts/freeze.py`, `scripts/cut-release.py` in pulumi/pulumi).

Keep this module standard-library-only -- it is imported by Lambdas before
the GitHub session is constructed.
"""

from __future__ import annotations

import re


_VERSION_RE = re.compile(r"^v?(\d+)\.(\d+)\.(\d+)(?:-(\S+))?$")


def parse(version: str) -> tuple[int, int, int, str | None]:
    """Parse 'v3.235.0' or '3.235.0-rc.1' into (major, minor, patch, suffix)."""
    m = _VERSION_RE.match(version.strip())
    if not m:
        raise ValueError(f"invalid version {version!r}; want X.Y.Z[-suffix]")
    return int(m.group(1)), int(m.group(2)), int(m.group(3)), m.group(4)


def is_valid(version: str) -> bool:
    return bool(_VERSION_RE.match(version.strip()))


def normalize(version: str) -> str:
    """Strip the 'v' prefix; return X.Y.Z[-suffix]."""
    major, minor, patch, suffix = parse(version)
    base = f"{major}.{minor}.{patch}"
    return f"{base}-{suffix}" if suffix else base


def with_v(version: str) -> str:
    """Add a 'v' prefix if missing."""
    return version if version.startswith("v") else f"v{version}"


def bump_minor(version: str) -> str:
    """Increment minor and reset patch (3.235.0 -> 3.236.0). Drops any suffix."""
    major, minor, _, _ = parse(version)
    return f"{major}.{minor + 1}.0"


def bump_patch(version: str) -> str:
    """Increment patch (3.216.1 -> 3.216.2). Drops any suffix."""
    major, minor, patch, _ = parse(version)
    return f"{major}.{minor}.{patch + 1}"


def compare(a: str, b: str) -> int:
    """-1 if a < b, 0 if equal, 1 if a > b. Suffixes order before any
    suffix-less version of the same X.Y.Z (so 1.2.3-rc.1 < 1.2.3)."""
    pa = parse(a); pb = parse(b)
    base_a = pa[:3]; base_b = pb[:3]
    if base_a != base_b:
        return -1 if base_a < base_b else 1
    if pa[3] is None and pb[3] is None:
        return 0
    if pa[3] is None:
        return 1
    if pb[3] is None:
        return -1
    return -1 if pa[3] < pb[3] else (0 if pa[3] == pb[3] else 1)
