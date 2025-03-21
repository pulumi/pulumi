from typing import TypedDict

import pulumi


class Args(TypedDict):
    rec: pulumi.Input["DoesntExist"]  # type: ignore


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
