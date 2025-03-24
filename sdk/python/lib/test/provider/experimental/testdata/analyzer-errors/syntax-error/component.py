from typing import TypedDict

import pulumi


class Args(TypedDict):
    a: str


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...

syntax error here
