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

from enum import Enum
from pulumi.resource import Resource
from pulumi.type_token import get_type_token, type_token


def test_type_token():
    class MyResourceWithoutToken(Resource): ...

    assert get_type_token(MyResourceWithoutToken) is None

    @type_token("package:module:resource")
    class MyResource(Resource): ...

    assert get_type_token(MyResource) == "package:module:resource"

    class MyEnumWithoutToken(Enum): ...

    assert get_type_token(MyEnumWithoutToken) is None

    @type_token("package:module:enum")
    class MyEnum(Enum): ...

    assert get_type_token(MyEnum) == "package:module:enum"
