from typing import TypedDict

import pulumi


class SkipMeArgs(TypedDict):
    foo: pulumi.Input[str]


class SkipMeComponent(pulumi.ComponentResource):
    def __init__(self, args: SkipMeArgs): ...


# We expect analyzer to skip the tests folder and not analyze this file.
