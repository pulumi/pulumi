from typing import List, Optional, Dict, TypedDict
import pulumi


class SubArgs(TypedDict):
    two_words: str
    optional_prop: Optional[str]
    input_prop: pulumi.Input[Optional[str]]


class ComplexSubArgs(TypedDict):
    one_item: SubArgs
    many_items: List[SubArgs]
    key_items: dict[str, SubArgs]


class MyComponentArgs(TypedDict):
    string_prop: str
    int_prop: pulumi.Input[int]
    list_prop: pulumi.Input[List[SubArgs]]
    object_prop: pulumi.Input[dict[str, SubArgs]]
    complex_prop: ComplexSubArgs


class MyComponent(pulumi.ComponentResource):
    def __init__(
        self,
        name: str,
        args: MyComponentArgs,
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        super().__init__("mycomp:index:MyComponent", name, {}, opts)
