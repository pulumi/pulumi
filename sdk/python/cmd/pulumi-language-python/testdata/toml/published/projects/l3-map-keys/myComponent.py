import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_primitive as primitive

class MyComponentArgs(TypedDict):
    booleanMap: Input[Dict[str, bool]]

class MyComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: MyComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:MyComponent", name, args, opts)

        res = primitive.Resource(f"{name}-res",
            boolean=False,
            float=2.17,
            integer=-12,
            string="adversarial",
            number_array=[
                float(0),
                float(1),
            ],
            boolean_map=args["booleanMap"],
            opts = pulumi.ResourceOptions(parent=self))

        self.boolean_map = res.boolean_map
        self.register_outputs({
            'booleanMap': res.boolean_map
        })