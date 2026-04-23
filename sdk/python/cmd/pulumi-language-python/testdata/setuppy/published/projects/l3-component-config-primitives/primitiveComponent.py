import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_primitive as primitive

class PrimitiveComponentArgs(TypedDict, total=False):
    boolean: Input[bool]
    float: Input[float]
    integer: Input[int]
    string: Input[str]

class PrimitiveComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: PrimitiveComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:PrimitiveComponent", name, args, opts)

        res = primitive.Resource(f"{name}-res",
            boolean=args["boolean"],
            float=args["float"],
            integer=args["integer"],
            string=args["string"],
            number_array=[
                float(-1),
                float(0),
                float(1),
            ],
            boolean_map={
                "t": True,
                "f": False,
            },
            opts = pulumi.ResourceOptions(parent=self))

        self.register_outputs({})
