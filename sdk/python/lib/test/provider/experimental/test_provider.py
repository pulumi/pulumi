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
        assert e.reason == "Missing required input 'MyComponent.a'"

    try:
        provider.map_inputs({"a": {}}, component_def)
        assert False, "expected an error"
    except InputPropertyError as e:
        assert e.reason == "Missing required input 'MyComponent.a.b'"

    try:
        provider.map_inputs({"a": {"b": {}}}, component_def)
        assert False, "expected an error"
    except InputPropertyError as e:
        assert e.reason == "Missing required input 'MyComponent.a.b.c'"
