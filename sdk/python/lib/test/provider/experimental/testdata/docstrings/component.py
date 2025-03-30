from enum import Enum
from typing import TypedDict
import pulumi


class Enu(Enum):
    """This is an enum"""

    A = "a"
    """Docstring for Enu.A"""


class NestedComplexType(TypedDict):
    """NestedComplexType doc string"""

    nested_value: pulumi.Input[str]
    """nested_value doc string"""


class ComplexType(TypedDict):
    # A comment and blank line before the description

    """ComplexType doc string"""

    value: str
    """value doc string"""

    another_value: pulumi.Input[NestedComplexType]


class Args(TypedDict):
    """Args doc string"""

    some_complex_type: pulumi.Input[ComplexType]

    """some_complex_type doc string"""

    input_with_comment_and_description: pulumi.Input[str]

    # A comment and blank line before the description

    """input_with_comment_and_description doc string"""

    enu: pulumi.Input[Enu]


class Component(pulumi.ComponentResource):
    """Component doc string"""

    complex_output: pulumi.Output[ComplexType]
    """complex_output doc string"""

    def __init__(self, args: Args): ...
