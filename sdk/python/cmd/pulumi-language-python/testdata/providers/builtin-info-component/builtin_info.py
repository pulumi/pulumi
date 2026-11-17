import pulumi
from typing import TypedDict

class BuiltinInfoArgs(TypedDict):
    pass

class BuiltinInfo(pulumi.ComponentResource):
    organization: pulumi.Output[str]
    project: pulumi.Output[str]
    stack: pulumi.Output[str]

    def __init__(self, name: str, args: BuiltinInfoArgs, opts: pulumi.ResourceOptions = None):
        super().__init__("builtin-info-component:index:BuiltinInfo", name, args, opts)

        self.organization = pulumi.Output.from_input(pulumi.get_organization())
        self.project = pulumi.Output.from_input(pulumi.get_project())
        self.stack = pulumi.Output.from_input(pulumi.get_stack())

        self.register_outputs({
            "organization": self.organization,
            "project": self.project,
            "stack": self.stack,
        })
