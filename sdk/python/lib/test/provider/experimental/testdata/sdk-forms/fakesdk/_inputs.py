# The input forms a generated SDK emits for an object type: the Args class, the TypedDict, and
# references to sibling types by name, which only resolve in this module.
from typing import Optional, Sequence, Union

import pulumi
from typing_extensions import NotRequired, TypedDict

from . import outputs

__all__ = ["InnerArgs", "InnerArgsDict", "OuterArgsDict"]


@pulumi.input_type
class InnerArgs:
    def __init__(__self__, *, n: pulumi.Input[int]):
        pulumi.set(__self__, "n", n)

    @property
    @pulumi.getter
    def n(self) -> pulumi.Input[int]:
        return pulumi.get(self, "n")


class InnerArgsDict(TypedDict):
    n: pulumi.Input[int]


class OuterArgsDict(TypedDict):
    inner: NotRequired[
        pulumi.Input[Union["InnerArgs", "InnerArgsDict", "outputs.Inner"]]
    ]
    inners: NotRequired[
        pulumi.Input[
            Sequence[pulumi.Input[Union["InnerArgs", "InnerArgsDict", "outputs.Inner"]]]
        ]
    ]
    maybe: Optional[pulumi.Input[Union["InnerArgs", "InnerArgsDict", "outputs.Inner"]]]
    by_name: NotRequired[pulumi.Input["InnerArgsDict"]]
