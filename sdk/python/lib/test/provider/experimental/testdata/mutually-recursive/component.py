from typing import Optional

import pulumi


class RecursiveTypeA:
    b: Optional[pulumi.Input["RecursiveTypeB"]]


class RecursiveTypeB:
    a: Optional[pulumi.Input[RecursiveTypeA]]


class Args:
    rec: pulumi.Input[RecursiveTypeA]


class Component(pulumi.ComponentResource):
    rec: pulumi.Output[RecursiveTypeA]

    def __init__(self, args: Args): ...
