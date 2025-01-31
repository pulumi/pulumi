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

from typing import Any, Optional, TypedDict, Union

import pulumi
from pulumi.provider.experimental.metadata import Metadata
from pulumi.provider.experimental.analyzer import Analyzer, unwrap_input, unwrap_output
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
    component = analyzer.analyze_component(SelfSignedCertificate)
    assert component == ComponentDefinition(
        description="Component doc string",
        inputs={
            "algorithm": PropertyDefinition(type=PropertyType.STRING),
            "ecdsa_curve": PropertyDefinition(type=PropertyType.STRING, optional=True),
            "bits": PropertyDefinition(type=PropertyType.INTEGER, optional=True),
        },
        outputs={
            "pem": PropertyDefinition(type=PropertyType.STRING),
            "private_key": PropertyDefinition(type=PropertyType.STRING),
            "ca_cert": PropertyDefinition(type=PropertyType.STRING),
        },
    )


def test_analyze_component_no_args():
    class NoArgs(pulumi.ComponentResource): ...

    analyzer = Analyzer(metadata)
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

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Empty)
    assert component == ComponentDefinition(
        inputs={},
        outputs={},
    )


def test_analyze_component_plain_types():
    class Args:
        input_int: int
        input_str: str
        input_float: float
        input_bool: bool

    class Empty(pulumi.ComponentResource):
        output_int: pulumi.Output[int]
        output_str: pulumi.Output[str]
        output_float: pulumi.Output[float]
        output_bool: pulumi.Output[bool]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Empty)
    assert component == ComponentDefinition(
        inputs={
            "input_int": PropertyDefinition(type=PropertyType.INTEGER),
            "input_str": PropertyDefinition(type=PropertyType.STRING),
            "input_float": PropertyDefinition(type=PropertyType.NUMBER),
            "input_bool": PropertyDefinition(type=PropertyType.BOOLEAN),
        },
        outputs={
            "output_int": PropertyDefinition(type=PropertyType.INTEGER),
            "output_str": PropertyDefinition(type=PropertyType.STRING),
            "output_float": PropertyDefinition(type=PropertyType.NUMBER),
            "output_bool": PropertyDefinition(type=PropertyType.BOOLEAN),
        },
    )


def test_analyze_component_complex_type():
    class ComplexType:
        value: pulumi.Input[str]
        optional_value: Optional[pulumi.Input[int]]

    class Args:
        some_complex_type: pulumi.Input[ComplexType]

    class Component(pulumi.ComponentResource):
        complex_output: pulumi.Output[ComplexType]

        def __init__(self, args: Args): ...

    analyzer = Analyzer(metadata)
    component = analyzer.analyze_component(Component)
    assert component == ComponentDefinition(
        inputs={
            "some_complex_type": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType"
            ),
        },
        outputs={
            "complex_output": PropertyDefinition(
                ref="#/types/my-component:index:ComplexType"
            )
        },
    )
    assert analyzer.type_definitions == {
        "ComplexType": TypeDefinition(
            name="ComplexType",
            type="object",
            properties={
                "value": PropertyDefinition(type=PropertyType.STRING),
                "optional_value": PropertyDefinition(
                    type=PropertyType.INTEGER, optional=True
                ),
            },
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
