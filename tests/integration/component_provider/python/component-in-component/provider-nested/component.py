from typing import TypedDict

import pulumi


class Args(TypedDict):
    str_input: pulumi.Input[str]


class NestedComponent(pulumi.ComponentResource):
    str_output: pulumi.Output[str]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("component:index:NestedComponent", name, {}, opts)
        self.str_output = pulumi.Output.from_input(args.get("str_input")).apply(
            lambda x: x.upper()
        )
        self.register_outputs(
            {
                "str_output": self.str_output,
            }
        )
