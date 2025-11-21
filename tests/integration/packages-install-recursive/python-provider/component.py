"""Python component that depends on typescript-a component."""
from typing import Any, Optional, TypedDict
import pulumi
import pulumi_random as random
import pulumi_typescript_a as typescript_a

class ComponentArgs(TypedDict):
    """Arguments for Component."""

    echo: pulumi.Input[Any]

class Component(pulumi.ComponentResource):
    """Python component that creates an Echo resource and a typescript-a component."""

    child_id: pulumi.Output[str]
    echo: pulumi.Output[Any]
    ts_a_child_id: pulumi.Output[str]

    def __init__(self,
                 name: str,
                 args: ComponentArgs,
                 opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__('python-provider:index:Component', name, {}, opts)

        self.echo = pulumi.Output.from_input(args.get("echo"))

        # Create our own Echo resource
        resource = random.RandomString(f'{name}-child', length=8, opts=pulumi.ResourceOptions(parent=self))
        self.child_id = resource.id

        # Create a typescript-a component to demonstrate the dependency
        ts_a_comp = typescript_a.Component(
            f'{name}-ts-a',
            typescript_a.ComponentArgs(echo=args.get("echo")),
            pulumi.ResourceOptions(parent=self)
        )
        self.ts_a_child_id = ts_a_comp.child_id

        self.register_outputs({
            'childId': self.child_id,
            'echo': self.echo,
            'tsAChildId': self.ts_a_child_id,
        })
