import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import builtins as _builtins
import pulumi_primitive as primitive

class ConvertedArgs(TypedDict):
    boolean: Input[_builtins.bool]
    float: Input[_builtins.float]
    integer: Input[_builtins.int]
    string: Input[_builtins.str]
    numberArray: Input[list[_builtins.float]]
    booleanMap: Input[Dict[_builtins.str, _builtins.bool]]

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
