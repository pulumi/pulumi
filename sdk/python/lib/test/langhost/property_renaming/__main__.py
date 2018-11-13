# Copyright 2016-2018, Pulumi Corporation.
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
from typing import Dict

import pulumi
from pulumi import CustomResource, Output, Input

def assert_eq(l, r):
    assert l == r


class TranslatedResource(CustomResource):
    """
    translate_input_property and translate_output_property provide hooks for resources to control the names of their own
    properties. The SDK will invoke each hook under the following circumstances:

    1. When preparing a RegisterResource RPC, the engine will call translate_input_property *recursively* on all
       property keys defined by a resource. The property name that translate_input_property returns will be the name of
       the property when sent to the engine.
    2. When returning from a RegisterResource RPC, the engine will call translate_output_property *recursively* on the
       data object returned from the engine. The property name that translate_output_property returns will be the name
       of the property on the Python object.

    This is used by providers to project their resource properties into Python using idiomatic snake case, while
    ensuring that providers themselves always speak over the RPC interface using camel case.
    """
    transformed_prop: Output[str]
    engine_output_prop: Output[str]
    recursive_prop: Output[Dict[str, str]]

    def __init__(self, name: str, prop: Input[str]) -> None:
        CustomResource.__init__(self, "test:index:TranslatedResource", name, {
            "transformed_prop": prop,
            "recursive_prop": {
                "recursive_key": "value",
                "recursive_output": None,
            },
            "engine_output_prop": None,
        })

    # Note: providers tend to implement these functions using lookup tables.
    def translate_input_property(self, prop: str) -> str:
        if prop == "transformed_prop":
            return "engineProp"

        if prop == "recursive_prop":
            return "recursiveProp"

        if prop == "recursive_key":
            return "recursiveKey"

        if prop == "recursive_output":
            return "recursiveOutput"

        if prop == "engine_output_prop":
            return "engineOutputProp"

        return prop

    def translate_output_property(self, prop: str) -> str:
        if prop == "engineProp":
            return "transformed_prop"

        if prop == "recursiveProp":
            return "recursive_prop"

        if prop == "recursiveKey":
            return "recursive_key"

        if prop == "recursiveOutput":
            return "recursive_output"


        if prop == "engineOutputProp":
            return "engine_output_prop"

        return prop


res = TranslatedResource("res", "some string")
res.transformed_prop.apply(lambda s: assert_eq(s, "some string"))
res.engine_output_prop.apply(lambda s: assert_eq(s, "some output string"))

pulumi.export("transformed_prop", res.transformed_prop)
pulumi.export("engine_output_prop", res.engine_output_prop)
pulumi.export("recursive_prop", res.recursive_prop)
