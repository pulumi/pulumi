#!/usr/bin/env python3

import os
import re
import subprocess
import sys
import json

def replace_line(lines, prefix, new_line):
    for i, line in enumerate(lines):
        if line.startswith(prefix):
            lines[i] = new_line
            return True
    return False

def main():
    if len(sys.argv) != 2:
        print("Usage: set-version.py <version>")
        sys.exit(1)

    version = sys.argv[1]

    if version.startswith("v"):
        print("Version should not start with v")
        sys.exit(1)

    with open("sdk/.version", "w+") as f:
        f.write(version + "\n")

    gomods = subprocess.run(["git", "ls-files", "*go.mod"], check=True, capture_output=True, text=True).stdout.split()
    for path in gomods:
        gomod = open(path).read()
        if path != "pkg/go.mod" and not re.search(r"github\.com/pulumi/pulumi/pkg/v3 => \.", gomod):
            continue
        gomod, n = re.subn(r"^((?:require)?\s+github\.com/pulumi/pulumi/sdk/v3) v\S+$", rf"\g<1> v{version}", gomod, flags=re.M)
        if n > 1 or (n == 0 and path == "pkg/go.mod"):
            print(f"Expected one github.com/pulumi/pulumi/sdk/v3 requirement in {path}, found {n}")
            sys.exit(1)
        with open(path, "w") as f:
            f.write(gomod)

    node = open("sdk/nodejs/package.json").readlines()
    replace_line(node, "    \"version\":", f'    "version": "{version}",\n')
    with open("sdk/nodejs/package.json", "w") as f:
        f.write("".join(node))

    node = open("sdk/nodejs/version.ts").readlines()
    replace_line(node, "export const version = ", f'export const version = "{version}";\n')
    with open("sdk/nodejs/version.ts", "w") as f:
        f.write("".join(node))

    npm = open("npm/package.json").readlines()
    replace_line(npm, "    \"version\":", f'    "version": "{version}",\n')
    with open("npm/package.json", "w") as f:
        f.write("".join(npm))

    python = open("sdk/python/lib/pulumi/_version.py").readlines()
    replace_line(python, "_VERSION = ", f'_VERSION = "{version}"\n')
    with open("sdk/python/lib/pulumi/_version.py", "w") as f:
        f.write("".join(python))
    pyproject = open("sdk/python/pyproject.toml").readlines()
    replace_line(pyproject, "version = ", f'version = "{version}"\n')
    with open("sdk/python/pyproject.toml", "w") as f:
        f.write("".join(pyproject))
    # Run `uv sync` to update the version in uv.lock
    subprocess.run(["uv", "sync"], cwd="sdk/python")

if __name__ == "__main__":
    main()
