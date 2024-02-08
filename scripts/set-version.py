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

    with open("sdk/.version", "w+") as f:
        f.write(version + "\n")

    node = open("sdk/nodejs/package.json").readlines()
    replace_line(node, "    \"version\":", f'    "version": "{version}",\n')
    with open("sdk/nodejs/package.json", "w") as f:
        f.write("".join(node))

    python = open("sdk/python/lib/setup.py").readlines()
    replace_line(python, "VERSION = ", f'VERSION = "{version}"\n')
    with open("sdk/python/lib/setup.py", "w") as f:
        f.write("".join(python))

if __name__ == "__main__":
    main()