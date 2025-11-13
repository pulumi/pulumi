import pulumi
from pulumi import ComponentResource, ResourceOptions, Input, Output
from typing import Optional

class SimpleComponentArgs:
    def __init__(self, message: Optional[Input[str]] = None):
        self.message = message

class SimpleComponent(ComponentResource):
    message: Output[str]

    def __init__(self, name: str, args: Optional[SimpleComponentArgs] = None, opts: Optional[ResourceOptions] = None):
        super().__init__("pkg-b:index:SimpleComponent", name, {}, opts)

        if args is None:
            args = SimpleComponentArgs()

        self.message = pulumi.Output.from_input(args.message if args.message else "Hello from pkg-b")

        self.register_outputs({
            "message": self.message,
        })

if __name__ == "__main__":
    from pulumi.provider.experimental import component_provider_host
    component_provider_host(name="pkg-b", components=[SimpleComponent])
