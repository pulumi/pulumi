from typing import TypedDict

import pulumi


class Args(TypedDict):
    rec: pulumi.Input[str]


class MyComponent(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
