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

import pulumi


class Nested(TypedDict):
    nested_str: str


class Complex(TypedDict):
    complex_str: pulumi.Input[str]
    nested: pulumi.Input[Nested]


class Args(TypedDict):
    str_input: pulumi.Input[str]
    optional_int_input: Optional[pulumi.Input[int]]
    complex_input: Optional[pulumi.Input[Complex]]
    list_input: pulumi.Input[list[str]]
    dict_input: pulumi.Input[dict[str, int]]


class MyComponent(pulumi.ComponentResource):
    str_output: pulumi.Output[str]
    optional_int_output: pulumi.Output[Optional[int]]
    complex_output: pulumi.Output[Optional[Complex]]
    list_output: pulumi.Output[list[str]]
    dict_output: pulumi.Output[dict[str, int]]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("component:index:MyComponent", name, {}, opts)
        self.str_output = pulumi.Output.from_input(args.get("str_input")).apply(
            lambda x: x.upper()
        )
        self.optional_int_output = pulumi.Output.from_input(
            args.get("optional_int_input", None)
        ).apply(lambda x: x * 2 if x else None)
        self.complex_output = pulumi.Output.from_input(
            {
                "complex_str": "complex_str_value",
                "nested": pulumi.Output.from_input(
                    {
                        "nested_str": "nested_str_value",
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
        self.register_outputs({})
