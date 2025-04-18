import sys
from pathlib import Path
from typing import Optional

from pulumi.provider import main

from .metadata import Metadata
from .provider import ComponentProvider

# Bail out if we're already hosting. This prevents recursion when the analyzer
# loads this file. It's usually good style to not run code at import time, and
# use `if __name__ == "__main__"`, but let's make sure we guard against this
is_hosting = False


def componentProviderHost(metadata: Optional[Metadata] = None):
    global is_hosting
    if is_hosting:
        return
    is_hosting = True
    path = Path(sys.argv[0])
    if metadata is None:
        metadata = Metadata(path.absolute().name, "0.0.1")
    main(ComponentProvider(metadata, path), sys.argv[1:])


def run_from_path(path: Path) -> None:
    metadata = Metadata(path.absolute().name, "0.0.1")
    main(ComponentProvider(metadata, path), sys.argv[2:])
