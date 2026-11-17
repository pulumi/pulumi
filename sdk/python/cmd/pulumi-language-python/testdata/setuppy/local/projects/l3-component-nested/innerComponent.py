import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import builtins as _builtins
import pulumi_simple as simple

class InnerComponentArgs(TypedDict):
    input: Input[_builtins.bool]

class InnerComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: InnerComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:InnerComponent", name, args, opts)

        res = simple.Resource(f"{name}-res", value=not args["input"],
        opts = pulumi.ResourceOptions(parent=self))

        self.output = res.value
        self.register_outputs({
            'output': res.value
        })