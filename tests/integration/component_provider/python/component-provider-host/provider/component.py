# Copyright 2025, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from typing import Optional, TypedDict
from enum import Enum

import pulumi


class Emu(Enum):
    A = "a"
    B = "b"


class Nested(TypedDict):
    """Deep nesting"""

    str_plain: str
    """A plain string"""


class Complex(TypedDict):
    """ComplexType is very complicated"""

    str_input: pulumi.Input[str]
    nested_input: pulumi.Input[Nested]


class Args(TypedDict):
    str_input: pulumi.Input[str]
    """This is a string input"""
    optional_int_input: Optional[pulumi.Input[int]]
    complex_input: Optional[pulumi.Input[Complex]]
    list_input: pulumi.Input[list[str]]
    dict_input: pulumi.Input[dict[str, int]]
    asset_input: pulumi.Input[pulumi.Asset]
    archive_input: pulumi.Input[pulumi.Archive]
    enum_input: pulumi.Input[Emu]


class MyComponent(pulumi.ComponentResource):
    """MyComponent is the best"""

    str_output: pulumi.Output[str]
    """This is a string output"""
    optional_int_output: Optional[pulumi.Output[int]]
    complex_output: Optional[pulumi.Output[Complex]]
    list_output: pulumi.Output[list[str]]
    dict_output: pulumi.Output[dict[str, int]]
    asset_output: pulumi.Output[pulumi.Asset]
    archive_output: pulumi.Output[pulumi.Archive]
    enum_output: pulumi.Output[Emu]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("component:index:MyComponent", name, {}, opts)
        self.str_output = pulumi.Output.from_input(args.get("str_input")).apply(
            lambda x: x.upper()
        )
        self.optional_int_output = pulumi.Output.from_input(
            args.get("optional_int_input", None)
        ).apply(lambda x: x * 2 if x else 7)
        self.complex_output = pulumi.Output.from_input(
            {
                "str_input": "complex_str_input_value",
                "nested_input": pulumi.Output.from_input(
                    {
                        "str_plain": "nested_str_plain_value",
                    }
                ),
            }
        )
        self.list_output = pulumi.Output.from_input(args.get("list_input")).apply(
            lambda x: [y.upper() for y in x]
        )
        self.dict_output = pulumi.Output.from_input(args.get("dict_input")).apply(
            lambda x: {k: v * 2 for k, v in x.items()}
        )
        self.asset_output = pulumi.Output.from_input(args.get("asset_input")).apply(
            self.transform_asset
        )
        self.archive_output = pulumi.Output.from_input(args.get("archive_input")).apply(
            self.transform_archive
        )
        self.enum_output = pulumi.Output.from_input(args.get("enum_input")).apply(
            self.transform_enum
        )
        self.register_outputs(
            {
                "asset_output": self.asset_output,
            }
        )

    def transform_asset(self, asset: pulumi.Asset) -> pulumi.Asset:
        if isinstance(asset, pulumi.StringAsset):
            return pulumi.StringAsset(asset.text.upper())
        elif isinstance(asset, pulumi.FileAsset):
            return pulumi.FileAsset(asset.path.upper())
        elif isinstance(asset, pulumi.RemoteAsset):
            return pulumi.RemoteAsset(asset.uri.upper())
        else:
            raise ValueError(f"Unexpected asset type {asset.__class__.__name__}")

    def transform_archive(self, asset: pulumi.Archive) -> pulumi.Archive:
        if isinstance(asset, pulumi.AssetArchive):
            asset = asset.assets["asset1"]  # type: ignore
            return pulumi.AssetArchive({"asset1": self.transform_asset(asset)})  # type: ignore
        else:
            raise ValueError(f"Unexpected archive type {asset.__class__.__name__}")

    def transform_enum(self, e: Emu) -> Emu:
        if e == Emu.A:
            return Emu.B
        elif e == Emu.B:
            return Emu.A
        raise Exception(f"Unexpected enum value: {e}")
