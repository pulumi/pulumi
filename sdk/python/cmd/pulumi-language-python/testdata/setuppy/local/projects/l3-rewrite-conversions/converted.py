import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_primitive as primitive

class ConvertedArgs(TypedDict, total=False):
    boolean: Input[bool]
    float: Input[float]
    integer: Input[int]
    string: Input[str]
    numberArray: Input[list[float]]
    booleanMap: Input[Dict[str, bool]]

class Converted(pulumi.ComponentResource):
    def __init__(self, name: str, args: ConvertedArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:Converted", name, args, opts)

        res = primitive.Resource(f"{name}-res",
            boolean=args["boolean"],
            float=args["float"],
            integer=args["integer"],
            string=args["string"],
            number_array=args["numberArray"],
            boolean_map=args["booleanMap"],
            opts = pulumi.ResourceOptions(parent=self))

        self.register_outputs({})
