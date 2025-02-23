from typing import TypedDict

import pulumi


class Args(TypedDict):
    foo: pulumi.Input[str]


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
