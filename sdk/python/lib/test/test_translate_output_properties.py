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
from typing import Dict, List, Optional

from pulumi.runtime import rpc
import pulumi

camel_case_to_snake_case = {
    "firstArg": "first_arg",
    "secondArg": "second_arg",
}


def translate_output_property(prop: str) -> str:
    return camel_case_to_snake_case.get(prop) or prop


@pulumi.output_type
class Foo(dict):
    first_arg: str = pulumi.property("firstArg")
    second_arg: float = pulumi.property("secondArg")

    def _translate_property(self, prop: str) -> str:
        return camel_case_to_snake_case.get(prop) or prop


@pulumi.output_type
class Bar(dict):
    third_arg: Foo = pulumi.property("thirdArg")
    third_optional_arg: Optional[Foo] = pulumi.property("thirdOptionalArg")

    fourth_arg: Dict[str, Foo] = pulumi.property("fourthArg")
    fourth_optional_arg: Dict[str, Optional[Foo]] = pulumi.property("fourthOptionalArg")

    fifth_arg: List[Foo] = pulumi.property("fifthArg")
    fifth_optional_arg: List[Optional[Foo]] = pulumi.property("fifthOptionalArg")

    sixth_arg: Dict[str, List[Foo]] = pulumi.property("sixthArg")
    sixth_optional_arg: Dict[str, Optional[List[Foo]]] = pulumi.property("sixthOptionalArg")
    sixth_optional_optional_arg: Dict[str, Optional[List[Optional[Foo]]]] = pulumi.property("sixthOptionalOptionalArg")

    seventh_arg: List[Dict[str, Foo]] = pulumi.property("seventhArg")
    seventh_optional_arg: List[Optional[Dict[str, Foo]]] = pulumi.property("seventhOptionalArg")
    seventh_optional_optional_arg: List[Optional[Dict[str, Optional[Foo]]]] = pulumi.property("seventhOptionalOptionalArg")

    eighth_arg: List[Dict[str, List[Foo]]] = pulumi.property("eighthArg")
    eighth_optional_arg: List[Optional[Dict[str, List[Foo]]]] = pulumi.property("eighthOptionalArg")
    eighth_optional_optional_arg: List[Optional[Dict[str, Optional[List[Foo]]]]] = pulumi.property("eighthOptionalOptionalArg")
    eighth_optional_optional_optional_arg: List[Optional[Dict[str, Optional[List[Optional[Foo]]]]]] = pulumi.property("eighthOptionalOptionalOptionalArg")

    def _translate_property(self, prop: str) -> str:
        return camel_case_to_snake_case.get(prop) or prop


@pulumi.output_type
class BarDeclared(dict):

    @property
    @pulumi.getter(name="thirdArg")
    def third_arg(self) -> Foo:
        ...

    @property
    @pulumi.getter(name="thirdOptionalArg")
    def third_optional_arg(self) -> Optional[Foo]:
        ...

    @property
    @pulumi.getter(name="fourthArg")
    def fourth_arg(self) -> Dict[str, Foo]:
        ...

    @property
    @pulumi.getter(name="fourthOptionalArg")
    def fourth_optional_arg(self) -> Dict[str, Optional[Foo]]:
        ...

    @property
    @pulumi.getter(name="fifthArg")
    def fifth_arg(self) -> List[Foo]:
        ...

    @property
    @pulumi.getter(name="fifthOptionalArg")
    def fifth_optional_arg(self) -> List[Optional[Foo]]:
        ...

    @property
    @pulumi.getter(name="sixthArg")
    def sixth_arg(self) -> Dict[str, List[Foo]]:
        ...

    @property
    @pulumi.getter(name="sixthOptionalArg")
    def sixth_optional_arg(self) -> Dict[str, Optional[List[Foo]]]:
        ...

    @property
    @pulumi.getter(name="sixthOptionalOptionalArg")
    def sixth_optional_optional_arg(self) -> Dict[str, Optional[List[Optional[Foo]]]]:
        ...

    @property
    @pulumi.getter(name="seventhArg")
    def seventh_arg(self) -> List[Dict[str, Foo]]:
        ...

    @property
    @pulumi.getter(name="seventhOptionalArg")
    def seventh_optional_arg(self) -> List[Optional[Dict[str, Foo]]]:
        ...

    @property
    @pulumi.getter(name="seventhOptionalOptionalArg")
    def seventh_optional_optional_arg(self) -> List[Optional[Dict[str, Optional[Foo]]]]:
        ...

    @property
    @pulumi.getter(name="eighthArg")
    def eighth_arg(self) -> List[Dict[str, List[Foo]]]:
        ...

    @property
    @pulumi.getter(name="eighthOptionalArg")
    def eighth_optional_arg(self) -> List[Optional[Dict[str, List[Foo]]]]:
        ...

    @property
    @pulumi.getter(name="eighthOptionalOptionalArg")
    def eighth_optional_optional_arg(self) -> List[Optional[Dict[str, Optional[List[Foo]]]]]:
        ...

    @property
    @pulumi.getter(name="eighthOptionalOptionalOptionalArg")
    def eighth_optional_optional_optional_arg(self) -> List[Optional[Dict[str, Optional[List[Optional[Foo]]]]]]:
        ...

    def _translate_property(self, prop: str) -> str:
        return camel_case_to_snake_case.get(prop) or prop


class TranslateOutputPropertiesTests(unittest.TestCase):
    def test_translate(self):
        output = {
            "firstArg": "hello",
            "secondArg": 42,
        }
        result = rpc.translate_output_properties(output, translate_output_property, Foo)
        self.assertIsInstance(result, Foo)
        self.assertEqual(result.first_arg, "hello")
        self.assertEqual(result["first_arg"], "hello")
        self.assertEqual(result.second_arg, 42)
        self.assertEqual(result["second_arg"], 42)

    def test_nested_types(self):
        def assertFoo(val, first_arg, second_arg):
            self.assertIsInstance(val, Foo)
            self.assertEqual(val.first_arg, first_arg)
            self.assertEqual(val["first_arg"], first_arg)
            self.assertEqual(val.second_arg, second_arg)
            self.assertEqual(val["second_arg"], second_arg)

        output = {
            "thirdArg": {
                "firstArg": "hello",
                "secondArg": 42,
            },
            "thirdOptionalArg": {
                "firstArg": "hello-opt",
                "secondArg": 142,
            },
            "fourthArg": {
                "foo": {
                    "firstArg": "hi",
                    "secondArg": 41,
                },
            },
            "fourthOptionalArg": {
                "foo": {
                    "firstArg": "hi-opt",
                    "secondArg": 141,
                },
            },
            "fifthArg": [{
                "firstArg": "bye",
                "secondArg": 40,
            }],
            "fifthOptionalArg": [{
                "firstArg": "bye-opt",
                "secondArg": 140,
            }],
            "sixthArg": {
                "bar": [{
                    "firstArg": "goodbye",
                    "secondArg": 39,
                }],
            },
            "sixthOptionalArg": {
                "bar": [{
                    "firstArg": "goodbye-opt",
                    "secondArg": 139,
                }],
            },
            "sixthOptionalOptionalArg": {
                "bar": [{
                    "firstArg": "goodbye-opt-opt",
                    "secondArg": 1139,
                }],
            },
            "seventhArg": [{
                "baz": {
                    "firstArg": "adios",
                    "secondArg": 38,
                },
            }],
            "seventhOptionalArg": [{
                "baz": {
                    "firstArg": "adios-opt",
                    "secondArg": 138,
                },
            }],
            "seventhOptionalOptionalArg": [{
                "baz": {
                    "firstArg": "adios-opt-opt",
                    "secondArg": 1138,
                },
            }],
            "eighthArg": [{
                "blah": [{
                    "firstArg": "farewell",
                    "secondArg": 37,
                }],
            }],
            "eighthOptionalArg": [{
                "blah": [{
                    "firstArg": "farewell-opt",
                    "secondArg": 137,
                }],
            }],
            "eighthOptionalOptionalArg": [{
                "blah": [{
                    "firstArg": "farewell-opt-opt",
                    "secondArg": 1137,
                }],
            }],
            "eighthOptionalOptionalOptionalArg": [{
                "blah": [{
                    "firstArg": "farewell-opt-opt-opt",
                    "secondArg": 11137,
                }],
            }],
        }

        for typ in [Bar, BarDeclared]:
            result = rpc.translate_output_properties(output, translate_output_property, typ)
            self.assertIsInstance(result, typ)

            self.assertIs(result.third_arg, result["thirdArg"])
            assertFoo(result.third_arg, "hello", 42)
            self.assertIs(result.third_optional_arg, result["thirdOptionalArg"])
            assertFoo(result.third_optional_arg, "hello-opt", 142)

            self.assertIs(result.fourth_arg, result["fourthArg"])
            assertFoo(result.fourth_arg["foo"], "hi", 41)
            self.assertIs(result.fourth_optional_arg, result["fourthOptionalArg"])
            assertFoo(result.fourth_optional_arg["foo"], "hi-opt", 141)

            self.assertIs(result.fifth_arg, result["fifthArg"])
            assertFoo(result.fifth_arg[0], "bye", 40)
            self.assertIs(result.fifth_optional_arg, result["fifthOptionalArg"])
            assertFoo(result.fifth_optional_arg[0], "bye-opt", 140)

            self.assertIs(result.sixth_arg, result["sixthArg"])
            assertFoo(result.sixth_arg["bar"][0], "goodbye", 39)
            self.assertIs(result.sixth_optional_arg, result["sixthOptionalArg"])
            assertFoo(result.sixth_optional_arg["bar"][0], "goodbye-opt", 139)
            self.assertIs(result.sixth_optional_optional_arg, result["sixthOptionalOptionalArg"])
            assertFoo(result.sixth_optional_optional_arg["bar"][0], "goodbye-opt-opt", 1139)

            self.assertIs(result.seventh_arg, result["seventhArg"])
            assertFoo(result.seventh_arg[0]["baz"], "adios", 38)
            self.assertIs(result.seventh_optional_arg, result["seventhOptionalArg"])
            assertFoo(result.seventh_optional_arg[0]["baz"], "adios-opt", 138)
            self.assertIs(result.seventh_optional_optional_arg, result["seventhOptionalOptionalArg"])
            assertFoo(result.seventh_optional_optional_arg[0]["baz"], "adios-opt-opt", 1138)

            self.assertIs(result.eighth_arg, result["eighthArg"])
            assertFoo(result.eighth_arg[0]["blah"][0], "farewell", 37)
            self.assertIs(result.eighth_optional_arg, result["eighthOptionalArg"])
            assertFoo(result.eighth_optional_arg[0]["blah"][0], "farewell-opt", 137)
            self.assertIs(result.eighth_optional_optional_arg, result["eighthOptionalOptionalArg"])
            assertFoo(result.eighth_optional_optional_arg[0]["blah"][0], "farewell-opt-opt", 1137)
            self.assertIs(result.eighth_optional_optional_optional_arg, result["eighthOptionalOptionalOptionalArg"])
            assertFoo(result.eighth_optional_optional_optional_arg[0]["blah"][0], "farewell-opt-opt-opt", 11137)
