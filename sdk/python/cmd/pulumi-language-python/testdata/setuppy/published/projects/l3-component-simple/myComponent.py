import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_simple as simple

class MyComponentArgs(TypedDict, total=False):
    input: Input[bool]

class MyComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: MyComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:MyComponent", name, args, opts)

        res = simple.Resource(f"{name}-res", value=args["input"],
        opts = pulumi.ResourceOptions(parent=self))

        self.output = res.value
        self.register_outputs({
            'output': res.value
        })