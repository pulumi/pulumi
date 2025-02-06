from typing import TypedDict

import pulumi


class MyDuplicateType(TypedDict):
    name: pulumi.Input[str]


class Args(TypedDict):
    rec: pulumi.Input[MyDuplicateType]


class MyComponentB(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
