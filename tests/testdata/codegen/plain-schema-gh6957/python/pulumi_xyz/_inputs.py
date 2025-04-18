# coding=utf-8
# *** WARNING: this file was generated by test. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

import builtins
import copy
import warnings
import sys
import pulumi
import pulumi.runtime
from typing import Any, Mapping, Optional, Sequence, Union, overload
if sys.version_info >= (3, 11):
    from typing import NotRequired, TypedDict, TypeAlias
else:
    from typing_extensions import NotRequired, TypedDict, TypeAlias
from . import _utilities

__all__ = [
    'FooArgs',
    'FooArgsDict',
]

MYPY = False

if not MYPY:
    class FooArgsDict(TypedDict):
        a: NotRequired[pulumi.Input[builtins.bool]]
elif False:
    FooArgsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class FooArgs:
    def __init__(__self__, *,
                 a: Optional[pulumi.Input[builtins.bool]] = None):
        if a is not None:
            pulumi.set(__self__, "a", a)

    @property
    @pulumi.getter
    def a(self) -> Optional[pulumi.Input[builtins.bool]]:
        return pulumi.get(self, "a")

    @a.setter
    def a(self, value: Optional[pulumi.Input[builtins.bool]]):
        pulumi.set(self, "a", value)


