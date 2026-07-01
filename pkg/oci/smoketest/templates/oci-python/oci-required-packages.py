#!/usr/bin/env python3
# oci-required-packages.py — generate the best-effort required-packages manifest baked
# into an OCI Python program image. It walks site-packages for `pulumi-plugin.json`
# files (the standalone metadata file Python/Go/.NET Pulumi SDKs carry — unlike Node,
# which embeds the same content in package.json) and writes a manifest
# {"plugins":[<PulumiPluginJSON>...]} to a well-known path. Each pulumi-plugin.json IS a
# PulumiPluginJSON, so entries are aggregated verbatim.
#
# This is template-owned and stdlib-only, run in the image build (Python is present).
#
# Best-effort, with a known imprecision worth noting: pulumi-language-python discovers
# plugins via importlib.metadata (installed distributions), then reads each package's
# pulumi-plugin.json. Walking site-packages instead finds the same files without the
# distribution bookkeeping; a faithful version would use importlib.metadata to scope to
# the program's actual dependency set.
#
# Usage: python3 oci-required-packages.py [site-packages-dir|AUTO] [output-path]
import json
import os
import sys
import sysconfig


def site_packages_root():
    # purelib is where pip installs packages (site-packages).
    return sysconfig.get_paths().get("purelib", "")


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "AUTO"
    if root == "AUTO":
        root = site_packages_root()
    out = sys.argv[2] if len(sys.argv) > 2 else ""

    # Dedup by (name, version): a package can surface from more than one path.
    found = {}
    for dirpath, _dirnames, filenames in os.walk(root):
        if "pulumi-plugin.json" not in filenames:
            continue
        try:
            with open(os.path.join(dirpath, "pulumi-plugin.json")) as f:
                data = json.load(f)
        except (OSError, ValueError):
            continue  # unreadable/malformed metadata is not worth failing the build over
        if data.get("resource") is not True:
            continue
        name = data.get("name")
        if not name:
            continue  # omit-over-wrong: a resource plugin with no resolvable name
        entry = {"resource": True, "name": name}
        if data.get("version"):
            entry["version"] = data["version"]
        if data.get("server"):
            entry["server"] = data["server"]
        if data.get("parameterization"):
            entry["parameterization"] = data["parameterization"]
        found[(name, data.get("version", ""))] = entry

    plugins = sorted(found.values(), key=lambda e: (e["name"], e.get("version", "")))
    manifest = json.dumps({"plugins": plugins}, indent=2) + "\n"

    if not out:
        sys.stdout.write(manifest)
        return
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w") as f:
        f.write(manifest)
    sys.stderr.write(f"oci-required-packages: wrote {len(plugins)} plugin(s) to {out}\n")


if __name__ == "__main__":
    main()
