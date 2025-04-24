# Copyright 2024, Pulumi Corporation.
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
import unittest.mock
import pulumi


class Resource1(pulumi.Resource):
    @property
    @pulumi.getter
    def foo(self) -> str:
        return "foo"

    @property
    @pulumi.getter
    @pulumi.deprecated("bar is deprecated; use foo instead")
    def bar(self) -> str:
        return "bar"


class DeprecatedTests(unittest.TestCase):
    def test_deprecated_can_be_called(self):
        # Arrange.
        r = Resource1("test", "test", True)
        expected = "bar"

        # Act.
        actual = r.bar

        # Assert.
        self.assertEqual(expected, actual)

    def test_deprecated_can_be_called_by_prop(self):
        # Arrange.
        prop = Resource1.__dict__["bar"]
        expected = "bar"

        # Act.
        actual = prop.fget(Resource1("test", "test", True))

        # Assert.
        self.assertEqual(expected, actual)

    def test_deprecated_is_tagged(self):
        # Arrange.
        prop = Resource1.__dict__["bar"]

        # Act.
        f = prop.fget.__dict__.get("_pulumi_deprecated_callable")

        # Assert.
        self.assertIsNotNone(f)

    def test_deprecated_can_passthrough(self):
        # Arrange.
        prop = Resource1.__dict__["bar"]
        f = prop.fget.__dict__.get("_pulumi_deprecated_callable")
        expected = "bar"

        # Act.
        actual = f(Resource1("test", "test", True))

        # Assert.
        self.assertEqual(expected, actual)

    @unittest.mock.patch("warnings.warn")
    def test_deprecated_prints_warnings(self, warnings_warn):
        # Arrange.
        prop = Resource1.__dict__["bar"]

        # Act.
        prop.fget(Resource1("test", "test", True))

        # Assert.
        warnings_warn.assert_called_once()

    def test_non_deprecated_can_be_called(self):
        # Arrange.
        r = Resource1("test", "test", True)
        expected = "foo"

        # Act.
        actual = r.foo

        # Assert.
        self.assertEqual(expected, actual)

    def test_non_deprecated_can_be_called_by_prop(self):
        # Arrange.
        prop = Resource1.__dict__["foo"]
        expected = "foo"

        # Act.
        actual = prop.fget(Resource1("test", "test", True))

        # Assert.
        self.assertEqual(expected, actual)

    def test_non_deprecated_is_not_tagged(self):
        # Arrange.
        prop = Resource1.__dict__["foo"]

        # Act.
        f = prop.fget.__dict__.get("_pulumi_deprecated_callable")

        # Assert.
        self.assertIsNone(f)
