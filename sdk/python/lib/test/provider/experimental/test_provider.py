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
