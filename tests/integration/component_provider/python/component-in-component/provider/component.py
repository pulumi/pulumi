from typing import TypedDict

import pulumi
import pulumi_provider_nested as provider_nested


class Args(TypedDict): ...


class MyComponent(pulumi.ComponentResource):
    str_output: pulumi.Output[str]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("component:index:MyComponent", name, {}, opts)
        nested = provider_nested.NestedComponent(
            "nested-component", str_input="Hello, Pulumi!"
        )
        self.str_output = nested.str_output
        self.register_outputs(
            {
                "str_output": nested.str_output,
            }
        )
