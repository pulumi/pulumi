import sys
from pathlib import Path
from typing import TypedDict

import pulumi

# Import the fake SDK package next to this file without importing any of its type names, so
# that the analyzer must resolve the SDK's forward references in the SDK's own modules.
sys.path.insert(0, str(Path(__file__).parent))
import fakesdk  # noqa: E402


class Args(TypedDict):
    outer: pulumi.Input[fakesdk.OuterArgsDict]


class Component(pulumi.ComponentResource):
    def __init__(self, name: str, args: Args): ...
