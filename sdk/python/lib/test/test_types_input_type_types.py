# Copyright 2016-2021, Pulumi Corporation.
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

from pulumi._types import input_type_types
import pulumi


@pulumi.input_type
class Foo:
    @property
    @pulumi.getter()
    def bar(self) -> pulumi.Input[str]:
        ...


@pulumi.input_type
class MySimpleInputType:
    a: str
    b: Optional[str]
    c: pulumi.Input[str]
    d: Optional[pulumi.Input[str]]
    e: Foo
    f: Optional[Foo]
    g: pulumi.Input[Foo]
    h: Optional[pulumi.Input[Foo]]
    i: pulumi.InputType[Foo]
    j: Optional[pulumi.InputType[Foo]]
    k: pulumi.Input[pulumi.InputType[Foo]]
    l: Optional[pulumi.Input[pulumi.InputType[Foo]]]


@pulumi.input_type
class MyPropertiesInputType:
    @property
    @pulumi.getter()
    def a(self) -> str:
        ...

    @property
    @pulumi.getter()
    def b(self) -> Optional[str]:
        ...

    @property
    @pulumi.getter()
    def c(self) -> pulumi.Input[str]:
        ...

    @property
    @pulumi.getter()
    def d(self) -> Optional[pulumi.Input[str]]:
        ...

    @property
    @pulumi.getter()
    def e(self) -> Foo:
        ...

    @property
    @pulumi.getter()
    def f(self) -> Optional[Foo]:
        ...

    @property
    @pulumi.getter()
    def g(self) -> pulumi.Input[Foo]:
        ...

    @property
    @pulumi.getter()
    def h(self) -> Optional[pulumi.Input[Foo]]:
        ...

    @property
    @pulumi.getter()
    def i(self) -> pulumi.InputType[Foo]:
        ...

    @property
    @pulumi.getter()
    def j(self) -> Optional[pulumi.InputType[Foo]]:
        ...

    @property
    @pulumi.getter()
    def k(self) -> pulumi.Input[pulumi.InputType[Foo]]:
        ...

    @property
    @pulumi.getter()
    def l(self) -> Optional[pulumi.Input[pulumi.InputType[Foo]]]:
        ...


class InputTypeTypesTests(unittest.TestCase):
    def test_input_type_types(self):
        expected = {
            "a": str,
            "b": str,
            "c": str,
            "d": str,
            "e": Foo,
            "f": Foo,
            "g": Foo,
            "h": Foo,
            "i": Foo,
            "j": Foo,
            "k": Foo,
            "l": Foo,
        }
        self.assertEqual(expected, input_type_types(MySimpleInputType))
        self.assertEqual(expected, input_type_types(MyPropertiesInputType))
