"""Normalizes paths to work with the quirks of Bash running on Windows
GitHub actions workers.

For example, on Windows:

    $ python realpath.py /d/a/pulumi/pulumi/nuget
    D:\a\pulumi\pulumi\nuget

"""

import os
import pathlib
import sys

path = sys.argv[1]

if os.name == 'nt':
    path = pathlib.PureWindowsPath(path)
    if path.root == '\\':
        # Assume drive is encoded as /d/x/full/path
        _, drive, rest = str(path).split('\\', 2)
        print(pathlib.PureWindowsPath(f'{drive}:\\', rest))
    else:
        print(path)
else:
    print(pathlib.PurePosixPath(path))
