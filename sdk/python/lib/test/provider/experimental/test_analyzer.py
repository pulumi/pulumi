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

import pytest
from enum import Enum
from importlib.machinery import SourceFileLoader
from collections import abc
import collections
from importlib.util import module_from_spec, spec_from_loader
from inspect import isclass
from pathlib import Path
import sys
import typing
from typing import Any, Optional, TypedDict, Union

import pulumi
from pulumi.provider.experimental.analyzer import (
    Analyzer,
    DuplicateTypeError,
    InvalidListTypeError,
    InvalidMapKeyError,
    InvalidMapTypeError,
    TypeNotFoundError,
    enum_value_type,
    is_dict,
    is_list,
    unwrap_input,
    unwrap_output,
)
from pulumi.provider.experimental.analyzer import (
    ComponentDefinition,
    Dependency,
    EnumValueDefinition,
    Parameterization,
    PropertyDefinition,
    PropertyType,
    TypeDefinition,
)
from pulumi.resource import ComponentResource


def test_analyze_component():
    class SelfSignedCertificateArgs(TypedDict):
        algorithm: pulumi.Input[str]
        ecdsa_curve: Optional[pulumi.Input[str]]
        bits: Optional[pulumi.Input[int]]

    class SelfSignedCertificate(pulumi.ComponentResource):
        """Component doc string"""

        pem: pulumi.Output[str]
        private_key: pulumi.Output[str]
        ca_cert: pulumi.Output[str]

        def __init__(self, args: SelfSignedCertificateArgs): ...

    analyzer = Analyzer("component")
    component = analyzer.analyze_component(SelfSignedCertificate)
    assert component == ComponentDefinition(
        name="SelfSignedCertificate",
        module="test_analyzer",
        description="Component doc string",
        inputs={
            "algorithm": PropertyDefinition(type=PropertyType.STRING),
            "ecdsaCurve": PropertyDefinition(type=PropertyType.STRING, optional=True),
            "bits": PropertyDefinition(type=PropertyType.INTEGER, optional=True),
        },
        inputs_mapping={
            "algorithm": "algorithm",
            "ecdsaCurve": "ecdsa_curve",
            "bits": "bits",
        },
        outputs={
            "pem": PropertyDefinition(type=PropertyType.STRING),
            "privateKey": PropertyDefinition(type=PropertyType.STRING),
            "caCert": PropertyDefinition(type=PropertyType.STRING),
        },
        outputs_mapping={
            "pem": "pem",
            "privateKey": "private_key",
            "caCert": "ca_cert",
        },
    )


def test_analyze_component_no_args():
    class NoArgs(pulumi.ComponentResource): ...

    analyzer = Analyzer("no-args")
    try:
        component = analyzer.analyze_component(NoArgs)
        assert False, f"expected an exception, got {component}"
    except Exception as e:
        assert (
            str(e)
            == "ComponentResource 'NoArgs' requires an argument named 'args' with a type annotation in its __init__ method"
        )


def test_analyze_component_empty():
    class Empty(pulumi.ComponentResource):
        def __init__(self, args: dict[str, Any]): ...

    analyzer = Analyzer("empty")
    component = analyzer.analyze_component(Empty)
    assert component == ComponentDefinition(
        name="Empty",
        module="test_analyzer",
        inputs={},
        inputs_mapping={},
        outputs={},
        outputs_mapping={},
    )


def test_analyze_component_plain_types():
    class ComplexTypeInput(TypedDict):
        a_input_list_str: Optional[pulumi.Input[list[str]]]
        a_str: str

    class ComplexTypeOutput(TypedDict):
        a_output_list_str: Optional[pulumi.Output[list[str]]]
        a_str: str

    class Args(TypedDict):
        a_int: int
        a_str: str
        a_float: float
        a_bool: bool
        a_optional: Optional[str]
        a_list: list[str]
        a_input_list: pulumi.Input[list[str]]
        a_list_input: list[pulumi.Input[str]]
        a_input_list_input: pulumi.Input[list[pulumi.Input[str]]]
        a_dict: dict[str, int]
        a_dict_input: dict[str, pulumi.Input[int]]
        a_input_dict: pulumi.Input[dict[str, int]]
        a_input_dict_input: pulumi.Input[dict[str, pulumi.Input[int]]]
        a_complex_type: ComplexTypeInput
        a_input_complex_type: pulumi.Input[ComplexTypeInput]

    class Component(pulumi.ComponentResource):
        a_int: int
        a_str: str
        a_float: float
        a_bool: bool
        a_optional: Optional[str]
        a_output_list: pulumi.Output[list[str]]
        a_output_complex: pulumi.Output[ComplexTypeOutput]
        a_optional_output_complex: Optional[pulumi.Output[ComplexTypeOutput]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("plain-types")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "aInt": PropertyDefinition(type=PropertyType.INTEGER, plain=True),
            "aStr": PropertyDefinition(type=PropertyType.STRING, plain=True),
            "aFloat": PropertyDefinition(type=PropertyType.NUMBER, plain=True),
            "aBool": PropertyDefinition(type=PropertyType.BOOLEAN, plain=True),
            "aOptional": PropertyDefinition(
                type=PropertyType.STRING, optional=True, plain=True
            ),
            "aList": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
                plain=True,
            ),
            "aInputList": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
            "aListInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING),
                plain=True,
            ),
            "aInputListInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING),
            ),
            "aDict": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
                plain=True,
            ),
            "aDictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(type=PropertyType.INTEGER),
                plain=True,
            ),
            "aInputDict": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
            "aInputDictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(type=PropertyType.INTEGER),
            ),
            "aComplexType": PropertyDefinition(
                ref="#/types/plain-types:index:ComplexTypeInput",
                plain=True,
            ),
            "aInputComplexType": PropertyDefinition(
                ref="#/types/plain-types:index:ComplexTypeInput"
            ),
        },
        inputs_mapping={
            "aInt": "a_int",
            "aStr": "a_str",
            "aFloat": "a_float",
            "aBool": "a_bool",
            "aOptional": "a_optional",
            "aList": "a_list",
            "aInputList": "a_input_list",
            "aInputListInput": "a_input_list_input",
            "aListInput": "a_list_input",
            "aDict": "a_dict",
            "aDictInput": "a_dict_input",
            "aInputDict": "a_input_dict",
            "aInputDictInput": "a_input_dict_input",
            "aComplexType": "a_complex_type",
            "aInputComplexType": "a_input_complex_type",
        },
        outputs={
            "aInt": PropertyDefinition(type=PropertyType.INTEGER),
            "aStr": PropertyDefinition(type=PropertyType.STRING),
            "aFloat": PropertyDefinition(type=PropertyType.NUMBER),
            "aBool": PropertyDefinition(type=PropertyType.BOOLEAN),
            "aOptional": PropertyDefinition(type=PropertyType.STRING, optional=True),
            "aOutputList": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING),
            ),
            "aOutputComplex": PropertyDefinition(
                ref="#/types/plain-types:index:ComplexTypeOutput",
            ),
            "aOptionalOutputComplex": PropertyDefinition(
                ref="#/types/plain-types:index:ComplexTypeOutput", optional=True
            ),
        },
        outputs_mapping={
            "aInt": "a_int",
            "aStr": "a_str",
            "aFloat": "a_float",
            "aBool": "a_bool",
            "aOptional": "a_optional",
            "aOutputList": "a_output_list",
            "aOutputComplex": "a_output_complex",
            "aOptionalOutputComplex": "a_optional_output_complex",
        },
    )
    assert analyzer.type_definitions == {
        "ComplexTypeInput": TypeDefinition(
            name="ComplexTypeInput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "aInputListStr": PropertyDefinition(
                    type=PropertyType.ARRAY,
                    items=PropertyDefinition(type=PropertyType.STRING, plain=True),
                    optional=True,
                ),
                "aStr": PropertyDefinition(type=PropertyType.STRING, plain=True),
            },
            properties_mapping={"aInputListStr": "a_input_list_str", "aStr": "a_str"},
            python_type=ComplexTypeInput,
        ),
        "ComplexTypeOutput": TypeDefinition(
            name="ComplexTypeOutput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "aOutputListStr": PropertyDefinition(
                    type=PropertyType.ARRAY,
                    items=PropertyDefinition(type=PropertyType.STRING),
                    optional=True,
                ),
                "aStr": PropertyDefinition(type=PropertyType.STRING),
            },
            properties_mapping={"aOutputListStr": "a_output_list_str", "aStr": "a_str"},
            python_type=ComplexTypeOutput,
        ),
    }


def test_analyze_optional_3_10_syntax():
    if sys.version_info < (3, 10):
        pytest.skip(f"requires Python 3.10 or above, running on {sys.version_info}")

    class Args(TypedDict):
        optional_syntax: str | None
        optional_typing: Optional[str]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("optional")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "optionalSyntax": PropertyDefinition(
                type=PropertyType.STRING,
                optional=True,
                plain=True,
            ),
            "optionalTyping": PropertyDefinition(
                type=PropertyType.STRING,
                optional=True,
                plain=True,
            ),
        },
        inputs_mapping={
            "optionalSyntax": "optional_syntax",
            "optionalTyping": "optional_typing",
        },
        outputs={},
        outputs_mapping={},
    )


def test_analyze_list_simple():
    class Args(TypedDict):
        list_input: pulumi.Input[list[str]]
        typing_list_input: pulumi.Input[typing.List[str]]
        abc_sequence_input: pulumi.Input[abc.Sequence[str]]

    class Component(pulumi.ComponentResource):
        list_output: Optional[pulumi.Output[list[Optional[str]]]]
        typing_list_output: pulumi.Output[typing.List[str]]
        abc_sequence_output: pulumi.Output[abc.Sequence[str]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("list-simple")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "listInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
            "typingListInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
            "abcSequenceInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
        },
        inputs_mapping={
            "listInput": "list_input",
            "typingListInput": "typing_list_input",
            "abcSequenceInput": "abc_sequence_input",
        },
        outputs={
            "listOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, optional=True),
                optional=True,
            ),
            "typingListOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING),
            ),
            "abcSequenceOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING),
            ),
        },
        outputs_mapping={
            "listOutput": "list_output",
            "typingListOutput": "typing_list_output",
            "abcSequenceOutput": "abc_sequence_output",
        },
    )


def test_analyze_list_complex():
    class ComplexTypeInput(TypedDict):
        name: Optional[pulumi.Input[list[str]]]

    class ComplexTypeOutput(TypedDict):
        name: Optional[pulumi.Output[list[str]]]

    class Args(TypedDict):
        list_input: pulumi.Input[list[ComplexTypeInput]]

    class Component(pulumi.ComponentResource):
        list_output: pulumi.Output[list[ComplexTypeOutput]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("list-complex")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "listInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(
                    ref="#/types/list-complex:index:ComplexTypeInput", plain=True
                ),
            )
        },
        inputs_mapping={"listInput": "list_input"},
        outputs={
            "listOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(
                    ref="#/types/list-complex:index:ComplexTypeOutput"
                ),
            )
        },
        outputs_mapping={"listOutput": "list_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexTypeInput": TypeDefinition(
            name="ComplexTypeInput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "name": PropertyDefinition(
                    type=PropertyType.ARRAY,
                    items=PropertyDefinition(type=PropertyType.STRING, plain=True),
                    optional=True,
                ),
            },
            properties_mapping={
                "name": "name",
            },
            python_type=ComplexTypeInput,
        ),
        "ComplexTypeOutput": TypeDefinition(
            name="ComplexTypeOutput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "name": PropertyDefinition(
                    type=PropertyType.ARRAY,
                    items=PropertyDefinition(type=PropertyType.STRING),
                    optional=True,
                ),
            },
            properties_mapping={
                "name": "name",
            },
            python_type=ComplexTypeOutput,
        ),
    }


def test_analyze_list_missing_type():
    class Args(TypedDict):
        bad_list: pulumi.Input[list]  # type: ignore

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("missing-type")
    try:
        analyzer.analyze_component(Component)
    except InvalidListTypeError as e:
        assert (
            str(e)
            == "list types must specify a type argument, got 'list' for 'Args.bad_list'"
        )


def test_analyze_dict_non_str_key():
    class Args(TypedDict):
        bad_dict: pulumi.Input[dict[int, str]]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("dict-non-str-key")
    try:
        analyzer.analyze_component(Component)
    except InvalidMapKeyError as e:
        assert str(e) == "map keys must be strings, got 'int' for 'Args.bad_dict'"


def test_analyze_dice_no_types():
    class Args(TypedDict):
        bad_dict: pulumi.Input[dict]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("dict-no-types")
    try:
        analyzer.analyze_component(Component)
    except InvalidMapTypeError as e:
        assert (
            str(e)
            == "map types must specify two type arguments, got 'dict' for 'Args.bad_dict'"
        )


def test_analyze_dict_simple():
    class Args(TypedDict):
        dict_input: pulumi.Input[dict[str, int]]
        typing_dict_input: pulumi.Input[typing.Dict[str, int]]
        abc_mapping_input: pulumi.Input[abc.Mapping[str, int]]

    class Component(pulumi.ComponentResource):
        dict_output: Optional[pulumi.Output[dict[str, Optional[int]]]]
        typing_dict_output: pulumi.Output[typing.Dict[str, int]]
        abc_mapping_output: pulumi.Output[abc.Mapping[str, int]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("dict-simple")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "dictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
            "typingDictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
            "abcMappingInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
        },
        inputs_mapping={
            "dictInput": "dict_input",
            "typingDictInput": "typing_dict_input",
            "abcMappingInput": "abc_mapping_input",
        },
        outputs={
            "dictOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, optional=True
                ),
                optional=True,
            ),
            "typingDictOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(type=PropertyType.INTEGER),
            ),
            "abcMappingOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(type=PropertyType.INTEGER),
            ),
        },
        outputs_mapping={
            "dictOutput": "dict_output",
            "typingDictOutput": "typing_dict_output",
            "abcMappingOutput": "abc_mapping_output",
        },
    )


def test_analyze_dict_complex():
    class ComplexTypeInput(TypedDict):
        name: Optional[pulumi.Input[dict[str, int]]]

    class ComplexTypeOutput(TypedDict):
        name: Optional[pulumi.Output[dict[str, int]]]

    class Args(TypedDict):
        dict_input: pulumi.Input[dict[str, ComplexTypeInput]]

    class Component(pulumi.ComponentResource):
        dict_output: Optional[pulumi.Output[dict[str, Optional[ComplexTypeOutput]]]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("dict-complex")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "dictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    ref="#/types/dict-complex:index:ComplexTypeInput", plain=True
                ),
            )
        },
        inputs_mapping={"dictInput": "dict_input"},
        outputs={
            "dictOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    ref="#/types/dict-complex:index:ComplexTypeOutput",
                    optional=True,
                ),
                optional=True,
            ),
        },
        outputs_mapping={"dictOutput": "dict_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexTypeInput": TypeDefinition(
            name="ComplexTypeInput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "name": PropertyDefinition(
                    type=PropertyType.OBJECT,
                    additional_properties=PropertyDefinition(
                        type=PropertyType.INTEGER,
                        plain=True,
                    ),
                    optional=True,
                ),
            },
            properties_mapping={
                "name": "name",
            },
            python_type=ComplexTypeInput,
        ),
        "ComplexTypeOutput": TypeDefinition(
            name="ComplexTypeOutput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "name": PropertyDefinition(
                    type=PropertyType.OBJECT,
                    additional_properties=PropertyDefinition(
                        type=PropertyType.INTEGER,
                    ),
                    optional=True,
                ),
            },
            properties_mapping={
                "name": "name",
            },
            python_type=ComplexTypeOutput,
        ),
    }


def test_analyze_component_complex_type():
    class ComplexType(TypedDict):
        value: pulumi.Input[str]
        optional_value: Optional[pulumi.Input[int]]

    class Args(TypedDict):
        some_complex_type: pulumi.Input[ComplexType]

    class Component(pulumi.ComponentResource):
        complex_output: pulumi.Output[ComplexType]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("complex-type")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "someComplexType": PropertyDefinition(
                ref="#/types/complex-type:index:ComplexType"
            ),
        },
        inputs_mapping={"someComplexType": "some_complex_type"},
        outputs={
            "complexOutput": PropertyDefinition(
                ref="#/types/complex-type:index:ComplexType"
            )
        },
        outputs_mapping={"complexOutput": "complex_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexType": TypeDefinition(
            name="ComplexType",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "value": PropertyDefinition(type=PropertyType.STRING),
                "optionalValue": PropertyDefinition(
                    type=PropertyType.INTEGER, optional=True
                ),
            },
            properties_mapping={
                "value": "value",
                "optionalValue": "optional_value",
            },
            python_type=ComplexType,
        )
    }


def test_analyze_archive():
    class Args(TypedDict):
        input_archive: pulumi.Input[pulumi.Archive]

    class Component(pulumi.ComponentResource):
        output_archive: pulumi.Input[pulumi.Archive]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("archive")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={"inputArchive": PropertyDefinition(ref="pulumi.json#/Archive")},
        inputs_mapping={"inputArchive": "input_archive"},
        outputs={"outputArchive": PropertyDefinition(ref="pulumi.json#/Archive")},
        outputs_mapping={"outputArchive": "output_archive"},
    )


def test_analyze_asset():
    class Args(TypedDict):
        input_archive: pulumi.Input[pulumi.Asset]

    class Component(pulumi.ComponentResource):
        output_archive: pulumi.Input[pulumi.Asset]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("asset")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={"inputArchive": PropertyDefinition(ref="pulumi.json#/Asset")},
        inputs_mapping={"inputArchive": "input_archive"},
        outputs={"outputArchive": PropertyDefinition(ref="pulumi.json#/Asset")},
        outputs_mapping={"outputArchive": "output_archive"},
    )


def test_analyze_any():
    class Args(TypedDict):
        input_any: pulumi.Input[Any]
        regular_any: Any

    class Component(pulumi.ComponentResource):
        output_any: pulumi.Input[Any]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("any")
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "inputAny": PropertyDefinition(ref="pulumi.json#/Any"),
            "regularAny": PropertyDefinition(ref="pulumi.json#/Any", plain=True),
        },
        inputs_mapping={"inputAny": "input_any", "regularAny": "regular_any"},
        outputs={"outputAny": PropertyDefinition(ref="pulumi.json#/Any")},
        outputs_mapping={"outputAny": "output_any"},
    )


def test_analyze_descriptions():
    analyzer = Analyzer("descriptions")
    (components, type_definitions, _) = analyzer.analyze(
        components=load_components(Path("testdata", "docstrings")),
    )
    assert components == {
        "Component": ComponentDefinition(
            description="Component doc string",
            name="Component",
            module="docstrings",
            inputs={
                "someComplexType": PropertyDefinition(
                    description="some_complex_type doc string",
                    ref="#/types/descriptions:index:ComplexType",
                ),
                "inputWithCommentAndDescription": PropertyDefinition(
                    description="input_with_comment_and_description doc string",
                    type=PropertyType.STRING,
                ),
                "enu": PropertyDefinition(
                    ref="#/types/descriptions:index:Enu",
                ),
            },
            inputs_mapping={
                "someComplexType": "some_complex_type",
                "inputWithCommentAndDescription": "input_with_comment_and_description",
                "enu": "enu",
            },
            outputs={
                "complexOutput": PropertyDefinition(
                    description="complex_output doc string",
                    ref="#/types/descriptions:index:ComplexTypeOutput",
                )
            },
            outputs_mapping={"complexOutput": "complex_output"},
        )
    }
    assert type_definitions == {
        "ComplexType": TypeDefinition(
            description="ComplexType doc string",
            name="ComplexType",
            module="docstrings",
            type=PropertyType.OBJECT,
            properties={
                "value": PropertyDefinition(
                    description="value doc string",
                    type=PropertyType.STRING,
                    plain=True,
                ),
                "anotherValue": PropertyDefinition(
                    ref="#/types/descriptions:index:NestedComplexType",
                    description=None,
                ),
            },
            properties_mapping={"value": "value", "anotherValue": "another_value"},
            python_type=load_type(Path("testdata", "docstrings"), "ComplexType"),
        ),
        "ComplexTypeOutput": TypeDefinition(
            description="ComplexTypeOutput doc string",
            name="ComplexTypeOutput",
            module="docstrings",
            type=PropertyType.OBJECT,
            properties={
                "value": PropertyDefinition(
                    description="value doc string",
                    type=PropertyType.STRING,
                ),
            },
            properties_mapping={"value": "value"},
            python_type=load_type(Path("testdata", "docstrings"), "ComplexTypeOutput"),
        ),
        "NestedComplexType": TypeDefinition(
            description="NestedComplexType doc string",
            name="NestedComplexType",
            module="docstrings",
            type=PropertyType.OBJECT,
            properties={
                "nestedValue": PropertyDefinition(
                    type=PropertyType.STRING,
                    description="nested_value doc string",
                )
            },
            properties_mapping={"nestedValue": "nested_value"},
            python_type=load_type(Path("testdata", "docstrings"), "NestedComplexType"),
        ),
        "Enu": TypeDefinition(
            name="Enu",
            module="docstrings",
            description="This is an enum",
            properties={},
            properties_mapping={},
            type=PropertyType.STRING,
            enum=[
                EnumValueDefinition(
                    name="A", value="a", description="Docstring for Enu.A"
                ),
            ],
            python_type=load_type(Path("testdata", "docstrings"), "Enu"),
        ),
    }


def test_analyze_resource_ref():
    analyzer = Analyzer("resource-ref")

    (component_defs, _, dependencies) = analyzer.analyze(
        components=load_components(Path("testdata", "resource-ref")),
    )
    assert component_defs == {
        "Component": ComponentDefinition(
            name="Component",
            module="resource_ref",
            inputs={
                "res": PropertyDefinition(
                    ref="/mock_package/v1.2.3/schema.json#/resources/mock_package:index:MyResource",
                ),
                "resPara": PropertyDefinition(
                    ref="/terraform-provider/v0.10.0/schema.json#/resources/parameterized:index:MyResource",
                ),
            },
            inputs_mapping={
                "res": "res",
                "resPara": "res_para",
            },
            outputs={},
            outputs_mapping={},
        )
    }
    assert sorted(dependencies, key=lambda d: d.name) == [
        Dependency(
            name="mock_package",
            version="1.2.3",
            downloadURL="example.com/download",
        ),
        Dependency(
            name="terraform-provider",
            version="0.10.0",
            parameterization=Parameterization(
                name="parameterized",
                version="0.2.2",
                value="eyJyZW1vdGUiOnsidXJsIjoicmVnaXN0cnkub3BlbnRvZnUub3JnL25ldGxpZnkvbmV0bGlmeSIsInZlcnNpb24iOiIwLjIuMiJ9fQ==",
            ),
        ),
    ]


def test_analyze_resource_ref_no_resource_type():
    class MyResource(pulumi.CustomResource):
        pass

    class Args(TypedDict):
        password: pulumi.Input[MyResource]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("resource-ref")
    try:
        analyzer.analyze_component(Component)
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e)
            == "Can not determine resource reference for type 'MyResource' used in 'Args.password': 'MyResource.pulumi_type' is not defined. "
            + "This may be due to an outdated version of 'test_analyzer'."
        )


def test_analyze_bad_type():
    analyzer = Analyzer("bad-type")

    try:
        analyzer.analyze(
            components=load_components(Path("testdata", "analyzer-errors", "bad-type")),
        )
        assert False, "expected an exception"
    except TypeNotFoundError as e:
        assert (
            str(e)
            == "Could not find the type 'DoesntExist'. Ensure it is defined in your source code or is imported."
        )


def test_analyze_union_type():
    analyzer = Analyzer("union-type")

    try:
        analyzer.analyze(
            components=load_components(
                Path("testdata", "analyzer-errors", "union-type")
            ),
        )
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e)
            == "Union types are not supported: found type 'typing.Union[str, int]' for 'Args.uni'"
        )


def test_analyze_union_type_3_10_syntax():
    if sys.version_info < (3, 10):
        pytest.skip(f"requires Python 3.10 or above, running on {sys.version_info}")

    analyzer = Analyzer("union-type-3-10-syntax")

    try:
        analyzer.analyze(
            components=load_components(
                Path("testdata", "analyzer-errors", "union-type-3-10-syntax")
            ),
        )
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e)
            == "Union types are not supported: found type 'str | int' for 'Args.uni'"
        )


def test_analyze_enum_type():
    class MyEnumStr(Enum):
        """string enum"""

        A = "a"
        B = "b"

    class MyEnumBool(Enum):
        """bool enum"""

        A = True
        B = False

    class MyEnumFloat(Enum):
        """float enum"""

        A = 1.1
        B = 2.2

    class MyEnumInt(Enum):
        """int enum"""

        A = 1
        B = 2

    class Args(TypedDict):
        enu_str: MyEnumStr
        enu_bool: MyEnumBool
        enu_float: MyEnumFloat
        enu_int: MyEnumInt

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer("enum")
    (component_defs, type_defs, _) = analyzer.analyze(components=[Component])
    assert component_defs == {
        "Component": ComponentDefinition(
            name="Component",
            module="test_analyzer",
            inputs={
                "enuStr": PropertyDefinition(
                    ref="#/types/enum:index:MyEnumStr",
                    plain=True,
                ),
                "enuBool": PropertyDefinition(
                    ref="#/types/enum:index:MyEnumBool",
                    plain=True,
                ),
                "enuFloat": PropertyDefinition(
                    ref="#/types/enum:index:MyEnumFloat",
                    plain=True,
                ),
                "enuInt": PropertyDefinition(
                    ref="#/types/enum:index:MyEnumInt",
                    plain=True,
                ),
            },
            inputs_mapping={
                "enuStr": "enu_str",
                "enuBool": "enu_bool",
                "enuFloat": "enu_float",
                "enuInt": "enu_int",
            },
            outputs={},
            outputs_mapping={},
        )
    }
    assert type_defs == {
        "MyEnumStr": TypeDefinition(
            name="MyEnumStr",
            description="string enum",
            module="test_analyzer",
            properties={},
            properties_mapping={},
            type=PropertyType.STRING,
            enum=[
                EnumValueDefinition(name="A", value="a"),
                EnumValueDefinition(name="B", value="b"),
            ],
            python_type=MyEnumStr,
        ),
        "MyEnumBool": TypeDefinition(
            name="MyEnumBool",
            description="bool enum",
            module="test_analyzer",
            properties={},
            properties_mapping={},
            type=PropertyType.BOOLEAN,
            enum=[
                EnumValueDefinition(name="A", value=True),
                EnumValueDefinition(name="B", value=False),
            ],
            python_type=MyEnumBool,
        ),
        "MyEnumFloat": TypeDefinition(
            name="MyEnumFloat",
            description="float enum",
            module="test_analyzer",
            properties={},
            properties_mapping={},
            type=PropertyType.NUMBER,
            enum=[
                EnumValueDefinition(name="A", value=1.1),
                EnumValueDefinition(name="B", value=2.2),
            ],
            python_type=MyEnumFloat,
        ),
        "MyEnumInt": TypeDefinition(
            name="MyEnumInt",
            description="int enum",
            module="test_analyzer",
            properties={},
            properties_mapping={},
            type=PropertyType.INTEGER,
            enum=[
                EnumValueDefinition(name="A", value=1),
                EnumValueDefinition(name="B", value=2),
            ],
            python_type=MyEnumInt,
        ),
    }


def test_analyze_syntax_error():
    analyzer = Analyzer("syntax-error")

    try:
        analyzer.analyze(
            components=load_components(
                Path("testdata", "analyzer-errors", "syntax-error")
            ),
        )
        assert False, "expected an exception"
    except Exception as e:
        import traceback

        stack = traceback.extract_tb(e.__traceback__)[:]
        # The error message can be slightly different depending on the Python version.
        assert "invalid syntax" in str(e) and "component.py, line 13)" in str(e)


def test_analyze_duplicate_type():
    analyzer = Analyzer("duplicate-type")

    try:
        analyzer.analyze(
            components=load_components(
                Path("testdata", "analyzer-errors", "duplicate-type"),
            ),
        )
        assert False, "expected an exception"
    except DuplicateTypeError as e:
        assert (
            str(e)
            == "Duplicate type 'MyDuplicateType': "
            + "orginally defined in 'duplicate_type.component_a', "
            + "but also found in 'duplicate_type.component_b'"
        )


def test_analyze_duplicate_components():
    analyzer = Analyzer("duplicate-components")

    try:
        analyzer.analyze(
            components=load_components(
                Path("testdata", "analyzer-errors", "duplicate-components"),
            ),
        )
        assert False, "expected an exception"
    except DuplicateTypeError as e:
        assert (
            str(e)
            == "Duplicate type 'MyComponent': "
            + "orginally defined in 'duplicate_components.component_a', "
            + "but also found in 'duplicate_components.component_b'"
        )


def test_analyze_no_components():
    analyzer = Analyzer("no-components")

    try:
        analyzer.analyze(components=[])
        assert False, "expected an exception"
    except Exception as e:
        assert str(e) == "No components found"


def test_analyze_component_self_recursive_complex_type():
    class RecursiveType(TypedDict):
        rec: Optional[pulumi.Input["RecursiveType"]]
        value: str

    class RecursiveTypeOutput(TypedDict):
        rec: Optional[pulumi.Output["RecursiveTypeOutput"]]
        value: str

    class Args(TypedDict):
        rec: pulumi.Input[RecursiveType]

    class Component(pulumi.ComponentResource):
        rec: pulumi.Output[RecursiveTypeOutput]

        def __init__(self, args: Args): ...

    analyzer = Analyzer("recursive")
    component = analyzer.analyze_component(Component)
    assert analyzer.type_definitions == {
        "RecursiveType": TypeDefinition(
            name="RecursiveType",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "rec": PropertyDefinition(
                    optional=True,
                    ref="#/types/recursive:index:RecursiveType",
                ),
                "value": PropertyDefinition(type=PropertyType.STRING, plain=True),
            },
            properties_mapping={"rec": "rec", "value": "value"},
            python_type=RecursiveType,
        ),
        "RecursiveTypeOutput": TypeDefinition(
            name="RecursiveTypeOutput",
            module="test_analyzer",
            type=PropertyType.OBJECT,
            properties={
                "rec": PropertyDefinition(
                    optional=True,
                    ref="#/types/recursive:index:RecursiveTypeOutput",
                ),
                "value": PropertyDefinition(type=PropertyType.STRING),
            },
            properties_mapping={"rec": "rec", "value": "value"},
            python_type=RecursiveTypeOutput,
        ),
    }
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={"rec": PropertyDefinition(ref="#/types/recursive:index:RecursiveType")},
        inputs_mapping={"rec": "rec"},
        outputs={
            "rec": PropertyDefinition(ref="#/types/recursive:index:RecursiveTypeOutput")
        },
        outputs_mapping={"rec": "rec"},
    )


def test_analyze_component_mutually_recursive_complex_types_file():
    analyzer = Analyzer("mutually-recursive")

    (components, type_definitions, _) = analyzer.analyze(
        components=load_components(Path("testdata", "mutually-recursive")),
    )
    assert type_definitions == {
        "RecursiveTypeA": TypeDefinition(
            name="RecursiveTypeA",
            module="mutually_recursive",
            type=PropertyType.OBJECT,
            properties={
                "b": PropertyDefinition(
                    optional=True,
                    ref="#/types/mutually-recursive:index:RecursiveTypeB",
                )
            },
            properties_mapping={"b": "b"},
            python_type=load_type(
                Path("testdata", "mutually-recursive"), "RecursiveTypeA"
            ),
        ),
        "RecursiveTypeB": TypeDefinition(
            name="RecursiveTypeB",
            module="mutually_recursive",
            type=PropertyType.OBJECT,
            properties={
                "a": PropertyDefinition(
                    optional=True,
                    ref="#/types/mutually-recursive:index:RecursiveTypeA",
                )
            },
            properties_mapping={"a": "a"},
            python_type=load_type(
                Path("testdata", "mutually-recursive"), "RecursiveTypeB"
            ),
        ),
    }
    assert components == {
        "Component": ComponentDefinition(
            name="Component",
            module="mutually_recursive",
            inputs={
                "rec": PropertyDefinition(
                    ref="#/types/mutually-recursive:index:RecursiveTypeA"
                )
            },
            inputs_mapping={"rec": "rec"},
            outputs={
                "rec": PropertyDefinition(
                    ref="#/types/mutually-recursive:index:RecursiveTypeA"
                )
            },
            outputs_mapping={"rec": "rec"},
        )
    }


def test_unwrap_output():
    str_output = pulumi.Output[str]
    unwrapped = unwrap_output(str_output)
    assert unwrapped == str

    union_output = pulumi.Output[Union[str, int]]
    unwrapped = unwrap_output(union_output)
    assert unwrapped == Union[str, int]

    try:
        not_output = pulumi.Input[str]
        unwrap_output(not_output)
        assert False, "expected an exception"
    except ValueError as e:
        assert "is not an output type" in str(e)


def test_unwrap_input():
    str_input = pulumi.Input[str]
    unwrapped = unwrap_input(str_input)
    assert unwrapped == str

    union_input = pulumi.Input[Union[str, int]]
    unwrapped = unwrap_input(union_input)
    assert unwrapped == Union[str, int]

    try:
        not_input = pulumi.Output[str]
        unwrap_input(not_input)
        assert False, "expected an exception"
    except ValueError as e:
        assert "is not an input type" in str(e)


def test_is_dict():
    assert is_dict(dict[str, int])
    assert is_dict(abc.Mapping[str, int])
    assert is_dict(abc.MutableMapping[str, int])
    assert is_dict(abc.MutableMapping[str, int])
    assert is_dict(collections.defaultdict[str, int])
    assert is_dict(collections.OrderedDict[str, int])
    assert is_dict(collections.UserDict[str, int])
    assert is_dict(typing.Dict[str, int])
    assert is_dict(typing.DefaultDict[str, int])
    assert is_dict(typing.OrderedDict[str, int])
    assert is_dict(typing.Mapping[str, int])
    assert is_dict(typing.MutableMapping[str, int])


def test_is_list():
    assert is_list(list[str])
    assert is_list(abc.Sequence[str])
    assert is_list(abc.MutableSequence[str])
    assert is_list(collections.UserList[str])
    assert is_list(typing.List[str])
    assert is_list(typing.Sequence[str])
    assert is_list(typing.MutableSequence[str])


def test_enum_value_type():
    class Enu(Enum):
        A = "A"
        B = "B"

    assert PropertyType.STRING == enum_value_type(Enu)

    class FloatEnum(Enum):
        A = 1.0
        B = 2.0

    assert PropertyType.NUMBER == enum_value_type(FloatEnum)

    class IntEnum(Enum):
        A = 1
        B = 2

    assert PropertyType.INTEGER == enum_value_type(IntEnum)

    class BoolEnum(Enum):
        A = True
        B = False

    assert PropertyType.BOOLEAN == enum_value_type(BoolEnum)

    class UnsupportedEnum(Enum):
        A = [1, 2, 3]
        B = [4, 5, 6]

    try:
        enum_value_type(UnsupportedEnum)
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e)
            == "Invalid type for enum value 'UnsupportedEnum.A': '<class 'list'>'. Supported enum value types are bool, str, float and int."
        )


def load_components(p: Path) -> list[type[ComponentResource]]:
    """
    Load all the components from `component.py` if present, or from `__init__.py`.
    """
    parent = Path(__file__).parent
    component_file = Path(parent, p, "component.py")
    init_file = Path(parent, p, "__init__.py")
    file_to_load = component_file if component_file.exists() else init_file
    mod_name = p.name.replace("-", "_")
    loader = SourceFileLoader(mod_name, str(file_to_load))
    spec = spec_from_loader(mod_name, loader)
    if not spec:
        raise Exception(f"failed to load {file_to_load}")
    mod = module_from_spec(spec)
    sys.modules[mod_name] = mod
    loader.exec_module(mod)
    components: list[type[ComponentResource]] = []
    for _, v in mod.__dict__.items():
        if isclass(v) and issubclass(v, ComponentResource):
            components.append(v)
    return components


def load_type(p: Path, type_name: str) -> type:
    mod_name = p.name.replace("-", "_")
    mod = sys.modules[mod_name]
    if not mod:
        raise Exception(
            f"failed to load {mod_name}. Expected {mod_name} to already be loaded."
        )
    return getattr(mod, type_name)
