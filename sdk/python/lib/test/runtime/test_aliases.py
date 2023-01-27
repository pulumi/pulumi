# Copyright 2023, Pulumi Corporation.
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

import pytest

from pulumi.runtime.resource import create_alias_spec
from pulumi.resource import Alias

@pytest.mark.asyncio
async def test_create_alias_spec_empty():
    empty_alias = Alias()
    alias_spec = await create_alias_spec(empty_alias)
    assert alias_spec.name == ""
    assert alias_spec.type == ""
    assert alias_spec.stack == ""
    assert alias_spec.project == ""
    assert alias_spec.parentUrn == ""
    assert alias_spec.noParent is False

@pytest.mark.asyncio
async def test_create_alias_spec_name_only():
    alias = Alias(name="Bucket")
    alias_spec = await create_alias_spec(alias)
    assert alias_spec.name == "Bucket"

@pytest.mark.asyncio
async def test_create_alias_spec_type_only():
    alias = Alias(type_="Bucket")
    alias_spec = await create_alias_spec(alias)
    assert alias_spec.type == "Bucket"