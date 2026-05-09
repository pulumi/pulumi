"""
ComputeBumpVersions Lambda.

Compares each language host's fresh `latest_tag` (from a Resurvey step)
against the version pinned in `scripts/get-language-providers.sh` on
pulumi/pulumi master. Returns the subset that needs a bump PR.

Input:
    {
      "language_hosts": [
        {"name": "dotnet", "repo": "pulumi/pulumi-dotnet", "latest_tag": "v3.105.0", ...},
        ...
      ]
    }

Output:
    {
      "bumps": [
        {"name": "dotnet", "current_pin": "v3.103.1", "new_version": "v3.105.0"},
        ...
      ]
    }
"""

from __future__ import annotations

import base64
import re
from typing import Any

from pulumi_release import gh, versions


SCRIPT_PATH = "scripts/get-language-providers.sh"
ENTRY_RE = re.compile(r'^\s*"([a-z]+)\s+(v[0-9.]+)(?:\s+\S+)?"\s*$', re.M)


def _read_pinned() -> dict[str, str]:
    resp = gh.get(f"repos/pulumi/pulumi/contents/{SCRIPT_PATH}?ref=master").json()
    content = base64.b64decode(resp["content"]).decode()
    pinned = {}
    for m in ENTRY_RE.finditer(content):
        pinned[m.group(1)] = m.group(2)
    return pinned


def handle(event: dict, context) -> dict[str, Any]:
    pinned = _read_pinned()
    bumps = []
    for host in event["language_hosts"]:
        name = host["name"]
        latest = host["latest_tag"]
        current = pinned.get(name)
        if current and versions.compare(versions.normalize(latest), versions.normalize(current)) > 0:
            bumps.append({
                "name": name,
                "repo": host["repo"],
                "current_pin": current,
                "new_version": latest,
            })
    return {"bumps": bumps}
