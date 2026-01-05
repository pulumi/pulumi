import pulumi
import pulumi_simple as simple
from typing import TypedDict

class SimpleArgs(TypedDict):
    value: pulumi.Input[bool]

class Simple(pulumi.ComponentResource):
    value: pulumi.Output[bool]

    def __init__(self, name: str, args: SimpleArgs, opts: pulumi.ResourceOptions = None):
        super().__init__("conformance-component:index:Simple", name, args, opts)

        self.value = pulumi.Output.from_input(args["value"])

        res = simple.Resource(f"{name}-child",
            value=self.value.apply(lambda v: not v),
            opts=pulumi.ResourceOptions(parent=self)
        )

        self.register_outputs({
            "value": self.value,
        })