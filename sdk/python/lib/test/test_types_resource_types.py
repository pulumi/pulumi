# Copyright 2016-2020, Pulumi Corporation.
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

import unittest

from pulumi._types import resource_types
import pulumi


class Resource1(pulumi.Resource):
    pass


class Resource2(pulumi.Resource):
    foo: pulumi.Output[str]


class Resource3(pulumi.Resource):
    nested: pulumi.Output["Nested"]


class Resource4(pulumi.Resource):
    nested_value: pulumi.Output["Nested"] = pulumi.property("nestedValue")


class Resource5(pulumi.Resource):
    @property
    @pulumi.getter
    def foo(self) -> pulumi.Output[str]: ...  # type: ignore


class Resource6(pulumi.Resource):
    @property
    @pulumi.getter
    def nested(self) -> pulumi.Output["Nested"]: ...  # type: ignore


class Resource7(pulumi.Resource):
    @property
    @pulumi.getter(name="nestedValue")
    def nested_value(self) -> pulumi.Output["Nested"]: ...  # type: ignore


class Resource8(pulumi.Resource):
    foo: pulumi.Output


class Resource9(pulumi.Resource):
    @property
    @pulumi.getter
    def foo(self) -> pulumi.Output: ...  # type: ignore


class Resource10(pulumi.Resource):
    foo: str


class Resource11(pulumi.Resource):
    @property
    @pulumi.getter
    def foo(self) -> str: ...  # type: ignore


class Resource12(pulumi.Resource):
    @property
    @pulumi.getter
    def foo(self): ...  # type: ignore


@pulumi.output_type
class Nested:
    first: str
    second: str


class ResourceTypesTests(unittest.TestCase):
    def test_resource_types(self):
        self.assertEqual({}, resource_types(Resource1))

        self.assertEqual({"foo": str}, resource_types(Resource2))
        self.assertEqual({"nested": Nested}, resource_types(Resource3))
        self.assertEqual({"nestedValue": Nested}, resource_types(Resource4))

        self.assertEqual({"foo": str}, resource_types(Resource5))
        self.assertEqual({"nested": Nested}, resource_types(Resource6))
        self.assertEqual({"nestedValue": Nested}, resource_types(Resource7))

        # Non-generic Output excluded from types.
        self.assertEqual({}, resource_types(Resource8))
        self.assertEqual({}, resource_types(Resource9))

        # Type annotations not using Output.
        self.assertEqual({"foo": str}, resource_types(Resource10))
        self.assertEqual({"foo": str}, resource_types(Resource11))

        # No return type annotation from the property getter.
        self.assertEqual({}, resource_types(Resource12))
