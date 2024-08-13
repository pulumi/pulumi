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


@pulumi.input_type
class MySimpleInputType:
    first_value: pulumi.Input[str] = pulumi.property("firstValue")
    second_value: Optional[pulumi.Input[float]] = pulumi.property(
        "secondValue", default=None
    )


@pulumi.input_type
class MyInputType:
    first_value: pulumi.Input[str] = pulumi.property("firstValue")
    second_value: Optional[pulumi.Input[float]] = pulumi.property("secondValue")

    def __init__(
        self,
        first_value: pulumi.Input[str],
        second_value: Optional[pulumi.Input[float]] = None,
    ):
        pulumi.set(self, "first_value", first_value)
        pulumi.set(self, "second_value", second_value)


@pulumi.input_type
class MyDeclaredPropertiesInputType:
    def __init__(
        self,
        first_value: pulumi.Input[str],
        second_value: Optional[pulumi.Input[float]] = None,
    ):
        pulumi.set(self, "first_value", first_value)
        pulumi.set(self, "second_value", second_value)

    # Property with empty getter/setter bodies.
    @property
    @pulumi.getter(name="firstValue")
    def first_value(self) -> pulumi.Input[str]:  # type: ignore
        """First value docstring."""
        ...

    @first_value.setter
    def first_value(self, value: pulumi.Input[str]): ...

    # Property with implementations.
    @property
    @pulumi.getter(name="secondValue")
    def second_value(self) -> Optional[pulumi.Input[float]]:
        """Second value docstring."""
        return pulumi.get(self, "second_value")

    @second_value.setter
    def second_value(self, value: Optional[pulumi.Input[float]]):
        pulumi.set(self, "second_value", value)


@pulumi.input_type
class DefaultArgs:
    a: pulumi.Input[str] = pulumi.property("a", default="foo")
    b: pulumi.Input[str] = pulumi.property("b", default="bar")
    c: Optional[pulumi.Input[str]] = pulumi.property("c", default=None)


class InputTypeTests(unittest.TestCase):
    def test_decorator_raises(self):
        with self.assertRaises(AssertionError) as cm:

            @pulumi.input_type
            @pulumi.input_type
            class Foo:
                pass

        with self.assertRaises(AssertionError) as cm:

            @pulumi.input_type
            @pulumi.output_type
            class Bar:
                pass

    def test_is_input_type(self):
        types = [
            MyInputType,
            MyDeclaredPropertiesInputType,
        ]
        for typ in types:
            self.assertTrue(_types.is_input_type(typ))
            self.assertEqual(True, typ._pulumi_input_type)

    def test_input_type(self):
        types = [
            (MySimpleInputType, False),
            (MyInputType, False),
            (MyDeclaredPropertiesInputType, True),
        ]
        for typ, has_doc in types:
            t = typ(first_value="hello", second_value=42)
            self.assertEqual("hello", t.first_value)
            self.assertEqual(42, t.second_value)
            t.first_value = "world"
            self.assertEqual("world", t.first_value)
            t.second_value = 500
            self.assertEqual(500, t.second_value)

            first = typ.first_value
            self.assertIsInstance(first, property)
            self.assertTrue(callable(first.fget))
            self.assertEqual("first_value", first.fget.__name__)
            self.assertEqual({"return": pulumi.Input[str]}, first.fget.__annotations__)
            if has_doc:
                self.assertEqual("First value docstring.", first.fget.__doc__)
            self.assertEqual("firstValue", first.fget._pulumi_name)
            self.assertTrue(callable(first.fset))
            self.assertEqual("first_value", first.fset.__name__)
            self.assertEqual({"value": pulumi.Input[str]}, first.fset.__annotations__)

            second = typ.second_value
            self.assertIsInstance(second, property)
            self.assertTrue(callable(second.fget))
            self.assertEqual("second_value", second.fget.__name__)
            self.assertEqual(
                {"return": Optional[pulumi.Input[float]]}, second.fget.__annotations__
            )
            if has_doc:
                self.assertEqual("Second value docstring.", second.fget.__doc__)
            self.assertEqual("secondValue", second.fget._pulumi_name)
            self.assertTrue(callable(second.fset))
            self.assertEqual("second_value", second.fset.__name__)
            self.assertEqual(
                {"value": Optional[pulumi.Input[float]]}, second.fset.__annotations__
            )

            self.assertEqual(
                {
                    "firstValue": "world",
                    "secondValue": 500,
                },
                _types.input_type_to_dict(t),
            )

            self.assertTrue(hasattr(t, "__eq__"))
            self.assertTrue(t.__eq__(t))
            self.assertTrue(t == t)
            self.assertFalse(t != t)
            self.assertFalse(t == "not equal")

            t2 = typ(first_value="world", second_value=500)
            self.assertTrue(t.__eq__(t2))
            self.assertTrue(t == t2)
            self.assertFalse(t != t2)

            self.assertEqual(
                {
                    "firstValue": "world",
                    "secondValue": 500,
                },
                _types.input_type_to_dict(t2),
            )

            t3 = typ(first_value="foo", second_value=1)
            self.assertFalse(t.__eq__(t3))
            self.assertFalse(t == t3)
            self.assertTrue(t != t3)

            self.assertEqual(
                {
                    "firstValue": "foo",
                    "secondValue": 1,
                },
                _types.input_type_to_dict(t3),
            )

    def test_input_type_defaults(self):
        d = DefaultArgs()
        self.assertEqual("foo", d.a)
        self.assertEqual("bar", d.b)
        self.assertEqual(None, d.c)
