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
from pulumi import _types


CAMEL_TO_SNAKE_CASE_TABLE = {
    "firstValue": "first_value",
    "secondValue": "second_value",
}


@pulumi.output_type
class MyOutputType:
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue", default=None)


@pulumi.output_type
class MyOutputTypeDict(dict):
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue", default=None)


@pulumi.output_type
class MyOutputTypeTranslated:
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue", default=None)

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop


@pulumi.output_type
class MyOutputTypeDictTranslated(dict):
    first_value: str = pulumi.property("firstValue")
    second_value: Optional[float] = pulumi.property("secondValue", default=None)

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop


@pulumi.output_type
class MyDeclaredPropertiesOutputType:
    def __init__(self, first_value: str, second_value: Optional[float] = None):
        pulumi.set(self, "first_value", first_value)
        if second_value is not None:
            pulumi.set(self, "second_value", second_value)

    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:  # type: ignore
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "second_value")


@pulumi.output_type
class MyDeclaredPropertiesOutputTypeDict(dict):
    def __init__(self, first_value: str, second_value: Optional[float] = None):
        pulumi.set(self, "first_value", first_value)
        if second_value is not None:
            pulumi.set(self, "second_value", second_value)

    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:  # type: ignore
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "second_value")


@pulumi.output_type
class MyDeclaredPropertiesOutputTypeTranslated:
    def __init__(self, first_value: str, second_value: Optional[float] = None):
        pulumi.set(self, "first_value", first_value)
        if second_value is not None:
            pulumi.set(self, "second_value", second_value)

    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:  # type: ignore
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "second_value")

    def _translate_property(self, prop):
        return CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop


@pulumi.output_type
class MyDeclaredPropertiesOutputTypeDictTranslated(dict):
    def __init__(self, first_value: str, second_value: Optional[float] = None):
        pulumi.set(self, "first_value", first_value)
        if second_value is not None:
            pulumi.set(self, "second_value", second_value)

    # Property with empty body.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> str:  # type: ignore
        """First value docstring."""
        ...

    # Property with implementation.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[float]:
        """Second value docstring."""
        return pulumi.get(self, "second_value")

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
            self.assertTrue(hasattr(typ, "__init__"))

    def test_output_type_types(self):
        self.assertEqual(
            {
                "firstValue": str,
                "secondValue": float,
            },
            _types.output_type_types(MyOutputType),
        )

    def test_output_type(self):
        types = [
            (MyOutputType, False),
            (MyOutputTypeDict, False),
            (MyOutputTypeTranslated, False),
            (MyOutputTypeDictTranslated, False),
            (MyDeclaredPropertiesOutputType, True),
            (MyDeclaredPropertiesOutputTypeDict, True),
            (MyDeclaredPropertiesOutputTypeTranslated, True),
            (MyDeclaredPropertiesOutputTypeDictTranslated, True),
        ]
        for typ, has_doc in types:
            self.assertTrue(hasattr(typ, "__init__"))
            t = _types.output_type_from_dict(
                typ, {"firstValue": "hello", "secondValue": 42}
            )
            self.assertEqual("hello", t.first_value)
            self.assertEqual(42, t.second_value)

            if isinstance(t, dict):
                self.assertEqual("hello", t["first_value"])
                self.assertEqual(42, t["second_value"])

            first = typ.first_value
            self.assertIsInstance(first, property)
            self.assertTrue(callable(first.fget))
            self.assertEqual("first_value", first.fget.__name__)
            self.assertEqual({"return": str}, first.fget.__annotations__)
            if has_doc:
                self.assertEqual("First value docstring.", first.fget.__doc__)
            self.assertEqual("firstValue", first.fget._pulumi_name)

            second = typ.second_value
            self.assertIsInstance(second, property)
            self.assertTrue(callable(second.fget))
            self.assertEqual("second_value", second.fget.__name__)
            self.assertEqual({"return": Optional[float]}, second.fget.__annotations__)
            if has_doc:
                self.assertEqual("Second value docstring.", second.fget.__doc__)
            self.assertEqual("secondValue", second.fget._pulumi_name)

            self.assertTrue(hasattr(t, "__eq__"))
            self.assertTrue(t.__eq__(t))
            self.assertTrue(t == t)
            self.assertFalse(t != t)
            self.assertFalse(t == "not equal")

            t2 = _types.output_type_from_dict(
                typ, {"firstValue": "hello", "secondValue": 42}
            )
            self.assertTrue(t.__eq__(t2))
            self.assertTrue(t == t2)
            self.assertFalse(t != t2)

            if isinstance(t2, dict):
                self.assertEqual("hello", t2["first_value"])
                self.assertEqual(42, t2["second_value"])

            t3 = _types.output_type_from_dict(
                typ, {"firstValue": "foo", "secondValue": 1}
            )
            self.assertFalse(t.__eq__(t3))
            self.assertFalse(t == t3)
            self.assertTrue(t != t3)

            if isinstance(t3, dict):
                self.assertEqual("foo", t3["first_value"])
                self.assertEqual(1, t3["second_value"])
