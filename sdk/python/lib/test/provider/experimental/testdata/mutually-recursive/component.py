from typing import Optional

import pulumi


class RecursiveTypeA:
    # Note `RecursiveTypeB` is not referenced in any annotations other than as a
    # ForwardRef. This forces the analyser to to use the `unresolved_forward_refs`
    # mechanism to resolve the type.
    b: Optional[pulumi.Input["RecursiveTypeB"]]


class RecursiveTypeB:
    a: Optional[pulumi.Input[RecursiveTypeA]]


class Args:
    rec: pulumi.Input[RecursiveTypeA]


class Component(pulumi.ComponentResource):
    rec: pulumi.Output[RecursiveTypeA]

    def __init__(self, args: Args): ...
