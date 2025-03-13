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

from pathlib import Path
from pulumi.errors import InputPropertyError
from pulumi.provider.experimental.metadata import Metadata
from pulumi.provider.experimental.provider import ComponentProvider


def test_validate_resource_type_invalid():
    for rt in ["not-valid", "not:valid", "pkg:not-valid-module:type", "pkg:index:"]:
        try:
            ComponentProvider.validate_resource_type("pkg", rt)
            assert False, f"expected {rt} to be invalid"
        except ValueError:
            pass


def test_validate_resource_type_valid():
    for rt in ["pkg:index:type", "pkg::type", "pkg:index:Type123"]:
        ComponentProvider.validate_resource_type("pkg", rt)


def test_map_inputs():
    provider = ComponentProvider(
        Metadata("test-provider", "0.0.1"),
        Path(Path(__file__).parent, "testdata", "missing-input"),
    )
    component_def = provider._component_defs["MyComponent"]  # type: ignore

    try:
        provider.map_inputs({}, component_def)
        assert False, "expected an error"
    except InputPropertyError as e:
        assert e.reason == "Missing required input 'a' on 'MyComponent'"

    try:
        provider.map_inputs({"a": {}}, component_def)
        assert False, "expected an error"
    except InputPropertyError as e:
        assert e.reason == "Missing required input 'a.b' on 'MyComponent'"

    try:
        provider.map_inputs({"a": {"b": {}}}, component_def)
        assert False, "expected an error"
    except InputPropertyError as e:
        assert e.reason == "Missing required input 'a.b.c' on 'MyComponent'"


def test_map_complex_inputs():
    provider = ComponentProvider(
        Metadata("test-provider", "0.0.1"),
        Path(Path(__file__).parent, "testdata", "complex-args"),
    )
    component_def = provider._component_defs["MyComponent"]  # type: ignore

    inputs = {
        "stringProp": "hello",
        "intProp": 42,
        "listProp": [
            {"twoWords": "bla", "inputProp": "list1opt"},
            {"twoWords": "value2"},
        ],
        "objectProp": {
            "key1": {"twoWords": "bla", "inputProp": "obj1opt"},
            "key2": {"twoWords": "value2"},
        },
        "complexProp": {
            "oneItem": {"twoWords": "one", "inputProp": "complex1opt"},
            "manyItems": [
                {
                    "twoWords": "many1",
                    "optionalProp": "many1opt",
                    "anyProp": "anything",
                },
                {"twoWords": "many2", "inputProp": "complex2opt"},
            ],
            "keyItems": {
                "key1": {"twoWords": "key1", "optionalProp": "key1opt"},
                "key2": {"twoWords": "key2", "inputProp": "complex3opt"},
            },
        },
    }

    mapped = provider.map_inputs(inputs, component_def)
    assert mapped == {
        "string_prop": "hello",
        "int_prop": 42,
        "list_prop": [
            {"two_words": "bla", "input_prop": "list1opt"},
            {"two_words": "value2"},
        ],
        "object_prop": {
            "key1": {"two_words": "bla", "input_prop": "obj1opt"},
            "key2": {"two_words": "value2"},
        },
        "complex_prop": {
            "one_item": {"two_words": "one", "input_prop": "complex1opt"},
            "many_items": [
                {
                    "two_words": "many1",
                    "optional_prop": "many1opt",
                    "any_prop": "anything",
                },
                {"two_words": "many2", "input_prop": "complex2opt"},
            ],
            "key_items": {
                "key1": {"two_words": "key1", "optional_prop": "key1opt"},
                "key2": {"two_words": "key2", "input_prop": "complex3opt"},
            },
        },
    }
