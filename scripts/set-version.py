#!/usr/bin/env python3

import os
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

    # Drop a marker so the on-merge gate can block any other PR from merging
    # between the freeze PR and the post-release changelog/go.mod PR (which
    # removes this file). Stage it eagerly so it can't be left out of the
    # freeze commit by accident.
    with open(".release-pending", "w") as f:
        f.write(f"v{version}\n")
    subprocess.run(["git", "add", ".release-pending"], check=True)

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

if __name__ == "__main__":
    main()
