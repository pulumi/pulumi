from enum import Enum
from typing import TypedDict

import pulumi


class MyEnum(Enum):
    A = "A"
    B = "B"


class Args(TypedDict):
    enu: MyEnum  # Enums are not supported yet


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
