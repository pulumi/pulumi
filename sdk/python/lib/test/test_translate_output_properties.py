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
    def __init__(self,
                 third_arg: Foo,
                 third_optional_arg: Optional[Foo],
                 fourth_arg: Dict[str, Foo],
                 fourth_optional_arg: Dict[str, Optional[Foo]],
                 fifth_arg: List[Foo],
                 fifth_optional_arg: List[Optional[Foo]],
                 sixth_arg: Dict[str, List[Foo]],
                 sixth_optional_arg: Dict[str, Optional[List[Foo]]],
                 sixth_optional_optional_arg: Dict[str, Optional[List[Optional[Foo]]]],
                 seventh_arg: List[Dict[str, Foo]],
                 seventh_optional_arg: List[Optional[Dict[str, Foo]]],
                 seventh_optional_optional_arg: List[Optional[Dict[str, Optional[Foo]]]],
                 eighth_arg: List[Dict[str, List[Foo]]],
                 eighth_optional_arg: List[Optional[Dict[str, List[Foo]]]],
                 eighth_optional_optional_arg: List[Optional[Dict[str, Optional[List[Foo]]]]],
                 eighth_optional_optional_optional_arg: List[Optional[Dict[str, Optional[List[Optional[Foo]]]]]]):
        pulumi.set(self, "third_arg", third_arg)
        pulumi.set(self, "third_optional_arg", third_optional_arg)
        pulumi.set(self, "fourth_arg", fourth_arg)
        pulumi.set(self, "fourth_optional_arg", fourth_optional_arg)
        pulumi.set(self, "fifth_arg", fifth_arg)
        pulumi.set(self, "fifth_optional_arg", fifth_optional_arg)
        pulumi.set(self, "sixth_arg", sixth_arg)
        pulumi.set(self, "sixth_optional_arg", sixth_optional_arg)
        pulumi.set(self, "sixth_optional_optional_arg", sixth_optional_optional_arg)
        pulumi.set(self, "seventh_arg", seventh_arg)
        pulumi.set(self, "seventh_optional_arg", seventh_optional_arg)
        pulumi.set(self, "seventh_optional_optional_arg", seventh_optional_optional_arg)
        pulumi.set(self, "eighth_arg", eighth_arg)
        pulumi.set(self, "eighth_optional_arg", eighth_optional_arg)
        pulumi.set(self, "eighth_optional_optional_arg", eighth_optional_optional_arg)
        pulumi.set(self, "eighth_optional_optional_optional_arg", eighth_optional_optional_optional_arg)

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


@pulumi.output_type
class InvalidTypeStr(dict):
    value: str = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredStr(dict):
    def __init__(self, value: str):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> str:
        ...

@pulumi.output_type
class InvalidTypeOptionalStr(dict):
    value: Optional[str] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredOptionalStr(dict):
    def __init__(self, value: Optional[str]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Optional[str]:
        ...

@pulumi.output_type
class InvalidTypeDictStr(dict):
    value: Dict[str, str] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredDictStr(dict):
    def __init__(self, value: Dict[str, str]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Dict[str, str]:
        ...

@pulumi.output_type
class InvalidTypeOptionalDictStr(dict):
    value: Optional[Dict[str, str]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredOptionalDictStr(dict):
    def __init__(self, value: Optional[Dict[str, str]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Optional[Dict[str, str]]:
        ...

@pulumi.output_type
class InvalidTypeDictOptionalStr(dict):
    value: Dict[str, Optional[str]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredDictOptionalStr(dict):
    def __init__(self, value: Dict[str, Optional[str]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Dict[str, Optional[str]]:
        ...

@pulumi.output_type
class InvalidTypeOptionalDictOptionalStr(dict):
    value: Optional[Dict[str, Optional[str]]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredOptionalDictOptionalStr(dict):
    def __init__(self, value: Optional[Dict[str, Optional[str]]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Optional[Dict[str, Optional[str]]]:
        ...

@pulumi.output_type
class InvalidTypeListStr(dict):
    value: List[str] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredListStr(dict):
    def __init__(self, value: List[str]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> List[str]:
        ...

@pulumi.output_type
class InvalidTypeOptionalListStr(dict):
    value: Optional[List[str]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredOptionalListStr(dict):
    def __init__(self, value: Optional[List[str]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Optional[List[str]]:
        ...

@pulumi.output_type
class InvalidTypeListOptionalStr(dict):
    value: List[Optional[str]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredListOptionalStr(dict):
    def __init__(self, value: List[Optional[str]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> List[Optional[str]]:
        ...

@pulumi.output_type
class InvalidTypeOptionalListOptionalStr(dict):
    value: Optional[List[Optional[str]]] = pulumi.property("value")

@pulumi.output_type
class InvalidTypeDeclaredOptionalListOptionalStr(dict):
    def __init__(self, value: Optional[List[Optional[str]]]):
        pulumi.set(self, "value", value)

    @property
    @pulumi.getter
    def value(self) -> Optional[List[Optional[str]]]:
        ...


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

        def convert_properties_to_secrets(output: dict) -> dict:
            return {k: {rpc._special_sig_key: rpc._special_secret_sig, "value": v } for k, v in output.items()}

        def run_test(output: dict):
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

        for typ in [Bar, BarDeclared]:
            run_test(output)
            run_test(convert_properties_to_secrets(output))

    def test_nested_types_raises(self):
        dict_value = {
            "firstArg": "hello",
            "secondArg": 42,
        }
        list_value = ["hello"]

        tests = [
            (InvalidTypeStr, dict_value),
            (InvalidTypeDeclaredStr, dict_value),
            (InvalidTypeOptionalStr, dict_value),
            (InvalidTypeDeclaredOptionalStr, dict_value),

            (InvalidTypeStr, list_value),
            (InvalidTypeDeclaredStr, list_value),
            (InvalidTypeOptionalStr, list_value),
            (InvalidTypeDeclaredOptionalStr, list_value),

            (InvalidTypeDictStr, {"foo": dict_value}),
            (InvalidTypeDeclaredDictStr, {"foo": dict_value}),
            (InvalidTypeOptionalDictStr, {"foo": dict_value}),
            (InvalidTypeDeclaredOptionalDictStr, {"foo": dict_value}),
            (InvalidTypeDictOptionalStr, {"foo": dict_value}),
            (InvalidTypeDeclaredDictOptionalStr, {"foo": dict_value}),
            (InvalidTypeOptionalDictOptionalStr, {"foo": dict_value}),
            (InvalidTypeDeclaredOptionalDictOptionalStr, {"foo": dict_value}),

            (InvalidTypeDictStr, {"foo": list_value}),
            (InvalidTypeDeclaredDictStr, {"foo": list_value}),
            (InvalidTypeOptionalDictStr, {"foo": list_value}),
            (InvalidTypeDeclaredOptionalDictStr, {"foo": list_value}),
            (InvalidTypeDictOptionalStr, {"foo": list_value}),
            (InvalidTypeDeclaredDictOptionalStr, {"foo": list_value}),
            (InvalidTypeOptionalDictOptionalStr, {"foo": list_value}),
            (InvalidTypeDeclaredOptionalDictOptionalStr, {"foo": list_value}),

            (InvalidTypeListStr, [dict_value]),
            (InvalidTypeDeclaredListStr, [dict_value]),
            (InvalidTypeOptionalListStr, [dict_value]),
            (InvalidTypeDeclaredOptionalListStr, [dict_value]),
            (InvalidTypeListOptionalStr, [dict_value]),
            (InvalidTypeDeclaredListOptionalStr, [dict_value]),
            (InvalidTypeOptionalListOptionalStr, [dict_value]),
            (InvalidTypeDeclaredOptionalListOptionalStr, [dict_value]),

            (InvalidTypeListStr, [list_value]),
            (InvalidTypeDeclaredListStr, [list_value]),
            (InvalidTypeOptionalListStr, [list_value]),
            (InvalidTypeDeclaredOptionalListStr, [list_value]),
            (InvalidTypeListOptionalStr, [list_value]),
            (InvalidTypeDeclaredListOptionalStr, [list_value]),
            (InvalidTypeOptionalListOptionalStr, [list_value]),
            (InvalidTypeDeclaredOptionalListOptionalStr, [list_value]),
        ]

        for typ, value in tests:
            outputs = [
                {"value": value},
                {"value": {rpc._special_sig_key: rpc._special_secret_sig, "value": value}},
            ]
            for output in outputs:
                with self.assertRaises(AssertionError):
                    rpc.translate_output_properties(output, translate_output_property, typ)
