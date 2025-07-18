# coding=utf-8
# *** WARNING: this file was generated by test. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

import builtins as _builtins
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
    'Bar',
]

@pulumi.output_type
class Bar(dict):
    @staticmethod
    def __key_warning(key: str):
        suggest = None
        if key == "has-a-hyphen":
            suggest = "has_a_hyphen"

        if suggest:
            pulumi.log.warn(f"Key '{key}' not found in Bar. Access the value via the '{suggest}' property getter instead.")

    def __getitem__(self, key: str) -> Any:
        Bar.__key_warning(key)
        return super().__getitem__(key)

    def get(self, key: str, default = None) -> Any:
        Bar.__key_warning(key)
        return super().get(key, default)

    def __init__(__self__, *,
                 has_a_hyphen: Optional[_builtins.str] = None):
        if has_a_hyphen is not None:
            pulumi.set(__self__, "has_a_hyphen", has_a_hyphen)

    @_builtins.property
    @pulumi.getter(name="has-a-hyphen")
    def has_a_hyphen(self) -> Optional[_builtins.str]:
        return pulumi.get(self, "has_a_hyphen")


