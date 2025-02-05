from typing import Optional, TypedDict

import pulumi


class RecursiveTypeA(TypedDict):
    # Note `RecursiveTypeB` is not referenced in any annotations other than as a
    # ForwardRef. This forces the analyser to to use the `unresolved_forward_refs`
    # mechanism to resolve the type.
    b: Optional[pulumi.Input["RecursiveTypeB"]]


class RecursiveTypeB(TypedDict):
    a: Optional[pulumi.Input[RecursiveTypeA]]


class Args(TypedDict):
    rec: pulumi.Input[RecursiveTypeA]


class Component(pulumi.ComponentResource):
    rec: pulumi.Output[RecursiveTypeA]

    def __init__(self, args: Args): ...
