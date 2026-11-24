import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import builtins as _builtins
import pulumi_primitive as primitive

class PrimitiveComponentArgs(TypedDict):
    numberArray: Input[list[_builtins.float]]
    booleanMap: Input[Dict[_builtins.str, _builtins.bool]]

class PrimitiveComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: PrimitiveComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:PrimitiveComponent", name, args, opts)

        res = primitive.Resource(f"{name}-res",
            boolean=True,
            float=3.5,
            integer=3,
            string="plain",
            number_array=args["numberArray"],
            boolean_map=args["booleanMap"],
            opts = pulumi.ResourceOptions(parent=self))

        self.register_outputs({})
