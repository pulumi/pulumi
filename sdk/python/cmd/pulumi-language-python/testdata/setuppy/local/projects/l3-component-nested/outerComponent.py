import pulumi
from innerComponent import InnerComponent
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import builtins as _builtins

class OuterComponentArgs(TypedDict):
    input: Input[_builtins.bool]

class OuterComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: OuterComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:OuterComponent", name, args, opts)

        inner_component = InnerComponent(f"{name}-innerComponent", {
            'input': not args["input"]},
            opts = pulumi.ResourceOptions(parent=self))

        self.output = inner_component.output
        self.register_outputs({
            'output': inner_component.output
        })