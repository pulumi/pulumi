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

from collections import abc
import collections
from pathlib import Path
import typing
from typing import Any, Optional, TypedDict, Union

import pulumi
from pulumi.provider.experimental.metadata import Metadata
from pulumi.provider.experimental.analyzer import (
    Analyzer,
    DuplicateTypeError,
    InvalidListTypeError,
    InvalidMapKeyError,
    InvalidMapTypeError,
    TypeNotFoundError,
    is_dict,
    is_list,
    unwrap_input,
    unwrap_output,
)
from pulumi.provider.experimental.component import (
    ComponentDefinition,
    PropertyDefinition,
    PropertyType,
    TypeDefinition,
)

metadata = Metadata("my-component", "0.0.1")


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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(SelfSignedCertificate, Path("test_analyzer"))
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

    analyzer = Analyzer(metadata)
    try:
        component = analyzer.analyze_component(NoArgs, Path("test_analyzer"))
        assert False, f"expected an exception, got {component}"
    except Exception as e:
        assert (
            str(e)
            == "ComponentResource 'NoArgs' requires an argument named 'args' with a type annotation in its __init__ method"
        )


def test_analyze_component_empty():
    class Empty(pulumi.ComponentResource):
        def __init__(self, args: dict[str, Any]): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Empty, Path("test_analyzer"))
    assert component == ComponentDefinition(
        name="Empty",
        module="test_analyzer",
        inputs={},
        inputs_mapping={},
        outputs={},
        outputs_mapping={},
    )


def test_analyze_component_plain_types():
    class ComplexType(TypedDict):
        a_input_list_str: Optional[pulumi.Input[list[str]]]
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
        a_complex_type: ComplexType
        a_input_complex_type: pulumi.Input[ComplexType]

    class Component(pulumi.ComponentResource):
        a_int: int
        a_str: str
        a_float: float
        a_bool: bool
        a_optional: Optional[str]
        a_output_list: pulumi.Output[list[str]]
        a_output_complex: pulumi.Output[ComplexType]
        a_optional_output_complex: Optional[pulumi.Output[ComplexType]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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
                plain=False,
            ),
            "aListInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=False),
                plain=True,
            ),
            "aInputListInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=False),
                plain=False,
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
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=False
                ),
                plain=True,
            ),
            "aInputDict": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
                plain=False,
            ),
            "aInputDictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=False
                ),
                plain=False,
            ),
            "aComplexType": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType",
                plain=True,
            ),
            "aInputComplexType": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType",
                plain=False,
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
            "aInt": PropertyDefinition(type=PropertyType.INTEGER, plain=False),
            "aStr": PropertyDefinition(type=PropertyType.STRING, plain=False),
            "aFloat": PropertyDefinition(type=PropertyType.NUMBER, plain=False),
            "aBool": PropertyDefinition(type=PropertyType.BOOLEAN, plain=False),
            "aOptional": PropertyDefinition(
                type=PropertyType.STRING, plain=True, optional=True
            ),
            "aOutputList": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
                plain=False,
            ),
            "aOutputComplex": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType", plain=False
            ),
            "aOptionalOutputComplex": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType", plain=False, optional=True
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
        "ComplexType": TypeDefinition(
            name="ComplexType",
            module="test_analyzer",
            type="object",
            properties={
                "aInputListStr": PropertyDefinition(
                    type=PropertyType.ARRAY,
                    items=PropertyDefinition(type=PropertyType.STRING, plain=True),
                    optional=True,
                ),
                "aStr": PropertyDefinition(type=PropertyType.STRING, plain=True),
            },
            properties_mapping={"aInputListStr": "a_input_list_str", "aStr": "a_str"},
        )
    }


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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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
                items=PropertyDefinition(
                    type=PropertyType.STRING, optional=True, plain=True
                ),
                optional=True,
            ),
            "typingListOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
            "abcSequenceOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(type=PropertyType.STRING, plain=True),
            ),
        },
        outputs_mapping={
            "listOutput": "list_output",
            "typingListOutput": "typing_list_output",
            "abcSequenceOutput": "abc_sequence_output",
        },
    )


def test_analyze_list_complex():
    class ComplexType(TypedDict):
        name: Optional[pulumi.Input[list[str]]]

    class Args(TypedDict):
        list_input: pulumi.Input[list[ComplexType]]

    class Component(pulumi.ComponentResource):
        list_output: pulumi.Output[list[ComplexType]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "listInput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(
                    ref="#/types/my-component:index:ComplexType", plain=True
                ),
            )
        },
        inputs_mapping={"listInput": "list_input"},
        outputs={
            "listOutput": PropertyDefinition(
                type=PropertyType.ARRAY,
                items=PropertyDefinition(
                    ref="#/types/my-component:index:ComplexType", plain=True
                ),
            )
        },
        outputs_mapping={"listOutput": "list_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexType": TypeDefinition(
            name="ComplexType",
            module="test_analyzer",
            type="object",
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
        )
    }


def test_analyze_list_missing_type():
    class Args(TypedDict):
        bad_list: pulumi.Input[list]  # type: ignore

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    try:
        analyzer.analyze_component(Component, Path("test_analyzer"))
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

    analyzer = Analyzer(metadata)
    try:
        analyzer.analyze_component(Component, Path("test_analyzer"))
    except InvalidMapKeyError as e:
        assert str(e) == "map keys must be strings, got 'int' for 'Args.bad_dict'"


def test_analyze_dice_no_types():
    class Args(TypedDict):
        bad_dict: pulumi.Input[dict]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    try:
        analyzer.analyze_component(Component, Path("test_analyzer"))
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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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
                    type=PropertyType.INTEGER, optional=True, plain=True
                ),
                optional=True,
            ),
            "typingDictOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
            "abcMappingOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    type=PropertyType.INTEGER, plain=True
                ),
            ),
        },
        outputs_mapping={
            "dictOutput": "dict_output",
            "typingDictOutput": "typing_dict_output",
            "abcMappingOutput": "abc_mapping_output",
        },
    )


def test_analyze_dict_complex():
    class ComplexType(TypedDict):
        name: Optional[pulumi.Input[dict[str, int]]]

    class Args(TypedDict):
        dict_input: pulumi.Input[dict[str, ComplexType]]

    class Component(pulumi.ComponentResource):
        dict_output: Optional[pulumi.Output[dict[str, Optional[ComplexType]]]]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "dictInput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    ref="#/types/my-component:index:ComplexType", plain=True
                ),
            )
        },
        inputs_mapping={"dictInput": "dict_input"},
        outputs={
            "dictOutput": PropertyDefinition(
                type=PropertyType.OBJECT,
                additional_properties=PropertyDefinition(
                    ref="#/types/my-component:index:ComplexType",
                    optional=True,
                    plain=True,
                ),
                optional=True,
            ),
        },
        outputs_mapping={"dictOutput": "dict_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexType": TypeDefinition(
            name="ComplexType",
            module="test_analyzer",
            type="object",
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
        )
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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "someComplexType": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType"
            ),
        },
        inputs_mapping={"someComplexType": "some_complex_type"},
        outputs={
            "complexOutput": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType"
            )
        },
        outputs_mapping={"complexOutput": "complex_output"},
    )
    assert analyzer.type_definitions == {
        "ComplexType": TypeDefinition(
            name="ComplexType",
            module="test_analyzer",
            type="object",
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
        )
    }


def test_analyze_archive():
    class Args(TypedDict):
        input_archive: pulumi.Input[pulumi.Archive]

    class Component(pulumi.ComponentResource):
        output_archive: pulumi.Input[pulumi.Archive]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
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
    analyzer = Analyzer(metadata)
    (components, type_definitions) = analyzer.analyze(
        Path(Path(__file__).parent, "testdata", "docstrings")
    )
    print(analyzer.docstrings)
    assert components == {
        "Component": ComponentDefinition(
            description="Component doc string",
            name="Component",
            module="component.py",
            inputs={
                "someComplexType": PropertyDefinition(
                    description="some_complex_type doc string",
                    ref="#/types/my-component:index:ComplexType",
                ),
                "inputWithCommentAndDescription": PropertyDefinition(
                    description="input_with_comment_and_description doc string",
                    type=PropertyType.STRING,
                ),
            },
            inputs_mapping={
                "someComplexType": "some_complex_type",
                "inputWithCommentAndDescription": "input_with_comment_and_description",
            },
            outputs={
                "complexOutput": PropertyDefinition(
                    description="complex_output doc string",
                    ref="#/types/my-component:index:ComplexType",
                )
            },
            outputs_mapping={"complexOutput": "complex_output"},
        )
    }
    assert type_definitions == {
        "ComplexType": TypeDefinition(
            description="ComplexType doc string",
            name="ComplexType",
            module="component.py",
            type="object",
            properties={
                "value": PropertyDefinition(
                    description="value doc string",
                    type=PropertyType.STRING,
                    plain=True,
                ),
                "anotherValue": PropertyDefinition(
                    ref="#/types/my-component:index:NestedComplexType",
                    description=None,
                ),
            },
            properties_mapping={"value": "value", "anotherValue": "another_value"},
        ),
        "NestedComplexType": TypeDefinition(
            description="NestedComplexType doc string",
            name="NestedComplexType",
            module="component.py",
            type="object",
            properties={
                "nestedValue": PropertyDefinition(
                    type=PropertyType.STRING,
                    description="nested_value doc string",
                )
            },
            properties_mapping={"nestedValue": "nested_value"},
        ),
    }


def test_analyze_resource_ref():
    class MyResource(pulumi.CustomResource): ...

    class Args(TypedDict):
        password: pulumi.Input[MyResource]

    class Component(pulumi.ComponentResource):
        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    try:
        analyzer.analyze_component(Component, Path("test_analyzer"))
    except Exception as e:
        assert (
            str(e)
            == "Resource references are not supported yet: found type 'MyResource' for 'Args.password'"
        )


def test_analyze_bad_type():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(Path(__file__).parent, "testdata", "analyzer-errors", "bad-type")
        )
        assert False, "expected an exception"
    except TypeNotFoundError as e:
        assert (
            str(e)
            == "Could not find the type 'DoesntExist'. Ensure it is defined in your source code or is imported."
        )


def test_analyze_union_type():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(Path(__file__).parent, "testdata", "analyzer-errors", "union-type")
        )
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e)
            == "Union types are not supported: found type 'typing.Union[str, int]' for 'Args.uni'"
        )


def test_analyze_enum_type():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(Path(__file__).parent, "testdata", "analyzer-errors", "enum-type")
        )
        assert False, "expected an exception"
    except Exception as e:
        assert (
            str(e) == "Enum types are not supported: found type 'MyEnum' for 'Args.enu'"
        )


def test_analyze_syntax_error():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(Path(__file__).parent, "testdata", "analyzer-errors", "syntax-error")
        )
        assert False, "expected an exception"
    except Exception as e:
        print(e)
        import traceback

        stack = traceback.extract_tb(e.__traceback__)[:]
        print(stack)
        assert (
            str(e)
            == "Failed to parse component.py: invalid syntax (<unknown>, line 13)"
        )


def test_analyze_duplicate_type():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(Path(__file__).parent, "testdata", "analyzer-errors", "duplicate-type")
        )
        assert False, "expected an exception"
    except DuplicateTypeError as e:
        assert (
            str(e)
            == "Duplicate type 'MyDuplicateType': "
            + "orginally defined in 'component_a.py', "
            + "but also found in 'component_b.py'"
        )


def test_analyze_duplicate_components():
    analyzer = Analyzer(metadata)

    try:
        analyzer.analyze(
            Path(
                Path(__file__).parent,
                "testdata",
                "analyzer-errors",
                "duplicate-components",
            )
        )
        assert False, "expected an exception"
    except DuplicateTypeError as e:
        assert (
            str(e)
            == "Duplicate type 'MyComponent': "
            + "orginally defined in 'component_a.py', "
            + "but also found in 'component_b.py'"
        )


def test_analyze_component_self_recursive_complex_type():
    class RecursiveType(TypedDict):
        rec: Optional[pulumi.Input["RecursiveType"]]

    class Args(TypedDict):
        rec: pulumi.Input[RecursiveType]

    class Component(pulumi.ComponentResource):
        rec: pulumi.Output[RecursiveType]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
    assert analyzer.type_definitions == {
        "RecursiveType": TypeDefinition(
            name="RecursiveType",
            module="test_analyzer",
            type="object",
            properties={
                "rec": PropertyDefinition(
                    optional=True,
                    ref="#/types/my-component:index:RecursiveType",
                )
            },
            properties_mapping={"rec": "rec"},
        ),
    }
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "rec": PropertyDefinition(ref="#/types/my-component:index:RecursiveType")
        },
        inputs_mapping={"rec": "rec"},
        outputs={
            "rec": PropertyDefinition(ref="#/types/my-component:index:RecursiveType")
        },
        outputs_mapping={"rec": "rec"},
    )


def test_analyze_component_mutually_recursive_complex_types_inline():
    class RecursiveTypeA(TypedDict):
        b: Optional[pulumi.Input["RecursiveTypeB"]]

    class RecursiveTypeB(TypedDict):
        a: Optional[pulumi.Input[RecursiveTypeA]]

    class Args(TypedDict):
        rec: pulumi.Input[RecursiveTypeA]

    class Component(pulumi.ComponentResource):
        rec: pulumi.Output[RecursiveTypeB]
        # rec: pulumi.Output["RecursiveTypeB"]
        # Using a forward ref instead here causes the test to fail because we
        # would never encounter the type as we walk the tree of types that
        # starts with the Component.
        # When doing full analysis via Analyser.analyze, we can handle this case.
        # See test_analyze_component_mutually_recursive_complex_types_file for
        # an example of this.

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component, Path("test_analyzer"))
    assert analyzer.type_definitions == {
        "RecursiveTypeA": TypeDefinition(
            name="RecursiveTypeA",
            module="test_analyzer",
            type="object",
            properties={
                "b": PropertyDefinition(
                    optional=True,
                    ref="#/types/my-component:index:RecursiveTypeB",
                )
            },
            properties_mapping={"b": "b"},
        ),
        "RecursiveTypeB": TypeDefinition(
            name="RecursiveTypeB",
            module="test_analyzer",
            type="object",
            properties={
                "a": PropertyDefinition(
                    optional=True,
                    ref="#/types/my-component:index:RecursiveTypeA",
                )
            },
            properties_mapping={"a": "a"},
        ),
    }
    assert component == ComponentDefinition(
        name="Component",
        module="test_analyzer",
        inputs={
            "rec": PropertyDefinition(ref="#/types/my-component:index:RecursiveTypeA")
        },
        inputs_mapping={"rec": "rec"},
        outputs={
            "rec": PropertyDefinition(ref="#/types/my-component:index:RecursiveTypeB")
        },
        outputs_mapping={"rec": "rec"},
    )


def test_analyze_component_mutually_recursive_complex_types_file():
    analyzer = Analyzer(metadata)

    (components, type_definitions) = analyzer.analyze(
        Path(Path(__file__).parent, "testdata", "mutually-recursive")
    )
    assert type_definitions == {
        "RecursiveTypeA": TypeDefinition(
            name="RecursiveTypeA",
            module="component.py",
            type="object",
            properties={
                "b": PropertyDefinition(
                    optional=True,
                    ref="#/types/my-component:index:RecursiveTypeB",
                )
            },
            properties_mapping={"b": "b"},
        ),
        "RecursiveTypeB": TypeDefinition(
            name="RecursiveTypeB",
            module="component.py",
            type="object",
            properties={
                "a": PropertyDefinition(
                    optional=True,
                    ref="#/types/my-component:index:RecursiveTypeA",
                )
            },
            properties_mapping={"a": "a"},
        ),
    }
    assert components == {
        "Component": ComponentDefinition(
            name="Component",
            module="component.py",
            inputs={
                "rec": PropertyDefinition(
                    ref="#/types/my-component:index:RecursiveTypeA"
                )
            },
            inputs_mapping={"rec": "rec"},
            outputs={
                "rec": PropertyDefinition(
                    ref="#/types/my-component:index:RecursiveTypeA"
                )
            },
            outputs_mapping={"rec": "rec"},
        )
    }


def test_analyze_component_excluded_files():
    analyzer = Analyzer(metadata)

    (components, type_definitions) = analyzer.analyze(
        Path(Path(__file__).parent, "testdata", "excluded-files")
    )
    assert components == {
        "Component": ComponentDefinition(
            name="Component",
            module="component.py",
            inputs={
                "foo": PropertyDefinition(
                    type=PropertyType.STRING,
                )
            },
            inputs_mapping={"foo": "foo"},
            outputs={},
            outputs_mapping={},
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
