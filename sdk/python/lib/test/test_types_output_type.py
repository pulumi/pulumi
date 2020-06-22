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
from typing import Optional

import pulumi
import pulumi._types as _types


CAMEL_TO_SNAKE_CASE_TABLE = {
    "firstValue": "first_value",
    "secondValue": "second_value",
}

@pulumi.output_type
class MyOutputType:
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue")

@pulumi.output_type
class MyOutputTypeDict(dict):
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue")

@pulumi.output_type
class MyOutputTypeTranslated:
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue")

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

@pulumi.output_type
class MyOutputTypeDictTranslated(dict):
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue")

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop


@pulumi.output_type
class MyDeclaredPropertiesOutputType:
    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "secondValue")

@pulumi.output_type
class MyDeclaredPropertiesOutputTypeDict(dict):
    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "secondValue")

@pulumi.output_type
class MyDeclaredPropertiesOutputTypeTranslated:
    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "secondValue")

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

@pulumi.output_type
class MyDeclaredPropertiesOutputTypeDictTranslated(dict):
    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "secondValue")

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop


class InputTypeTests(unittest.TestCase):
    def test_decorator_raises(self):
        with self.assertRaises(AssertionError) as cm:
            @pulumi.output_type
            @pulumi.input_type
            class Foo:
                pass

        with self.assertRaises(AssertionError) as cm:
            @pulumi.output_type
            @pulumi.input_type
            class Bar:
                pass

    def test_is_output_type(self):
        types = [
            MyOutputType,
            MyOutputTypeDict,
            MyOutputTypeTranslated,
            MyOutputTypeDictTranslated,
            MyDeclaredPropertiesOutputType,
            MyDeclaredPropertiesOutputTypeDict,
            MyDeclaredPropertiesOutputTypeTranslated,
            MyDeclaredPropertiesOutputTypeDictTranslated,
        ]
        for typ in types:
            self.assertTrue(_types.is_output_type(typ))
            self.assertEqual(True, typ._pulumi_output_type)

    def test_output_type_types(self):
        self.assertEqual({
            "firstValue": str,
            "secondValue": float,
        }, _types.output_type_types(MyOutputType))

    def test_output_type(self):
        types = [
            (MyOutputType, "firstValue", "secondValue", False),
            (MyOutputTypeDict, "firstValue", "secondValue", False),
            (MyOutputTypeTranslated, "first_value", "second_value", False),
            (MyOutputTypeDictTranslated, "first_value", "second_value", False),
            (MyDeclaredPropertiesOutputType, "firstValue", "secondValue", True),
            (MyDeclaredPropertiesOutputTypeDict, "firstValue", "secondValue", True),
            (MyDeclaredPropertiesOutputTypeTranslated, "first_value", "second_value", True),
            (MyDeclaredPropertiesOutputTypeDictTranslated, "first_value", "second_value", True),
        ]
        for typ, k1, k2, has_doc in types:
            self.assertTrue(hasattr(MyOutputType, "__init__"))
            t = typ({k1: "hello", k2: 42})
            self.assertEqual("hello", t.first_value)
            self.assertEqual(42, t.second_value)

            if isinstance(t, dict):
                self.assertEqual("hello", t[k1])
                self.assertEqual(42, t[k2])

            first = typ.first_value
            self.assertIsInstance(first, property)
            self.assertTrue(callable(first.fget))
            self.assertEqual("first_value", first.fget.__name__)
            self.assertEqual({"return": str}, first.fget.__annotations__)
            if has_doc:
                self.assertEqual("First value docstring.", first.fget.__doc__)
            self.assertEqual(True, first.fget._pulumi_getter)
            self.assertEqual("firstValue", first.fget._pulumi_name)

            second = typ.second_value
            self.assertIsInstance(second, property)
            self.assertTrue(callable(second.fget))
            self.assertEqual("second_value", second.fget.__name__)
            self.assertEqual({"return": Optional[float]}, second.fget.__annotations__)
            if has_doc:
                self.assertEqual("Second value docstring.", second.fget.__doc__)
            self.assertEqual(True, second.fget._pulumi_getter)
            self.assertEqual("secondValue", second.fget._pulumi_name)

            self.assertTrue(hasattr(t, "__eq__"))
            self.assertTrue(t.__eq__(t))
            self.assertTrue(t == t)
            self.assertFalse(t != t)
            self.assertFalse(t == "not equal")

            t2 = typ({k1: "hello", k2: 42})
            self.assertTrue(t.__eq__(t2))
            self.assertTrue(t == t2)
            self.assertFalse(t != t2)

            if isinstance(t2, dict):
                self.assertEqual("hello", t2[k1])
                self.assertEqual(42, t2[k2])

            t3 = typ({k1: "foo", k2: 1})
            self.assertFalse(t.__eq__(t3))
            self.assertFalse(t == t3)
            self.assertTrue(t != t3)

            if isinstance(t3, dict):
                self.assertEqual("foo", t3[k1])
                self.assertEqual(1, t3[k2])
