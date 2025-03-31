import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_simple as simple

class SubmoduleArgs(TypedDict, total=False):
    submoduleListVar: Input[list[str]]
    submoduleFilterCond: Input[bool]
    submoduleFilterVariable: Input[int]

class Submodule(pulumi.ComponentResource):
    def __init__(self, name: str, args: SubmoduleArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:Submodule", name, args, opts)

        submodule_res = []
def create_submodule_res(range_body):
            for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
                submodule_res.append(simple.Resource(f"{name}-submoduleRes-{range['key']}", value=True,
                opts = pulumi.ResourceOptions(parent=self)))

pulumi.Output.from_input(args["submoduleListVar"]).apply(lambda resolved_outputs: create_submodule_res(        {k: v for k, v in resolved_outputs['to_output'] if pulumi.Output.from_input(args["submoduleFilterCond"])}))

        submodule_res_with_apply_filter = []
def create_submodule_res_with_apply_filter(range_body):
            for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
                submodule_res_with_apply_filter.append(simple.Resource(f"{name}-submoduleResWithApplyFilter-{range['key']}", value=True,
                opts = pulumi.ResourceOptions(parent=self)))

pulumi.Output.all(
            to_output=pulumi.Output.from_input(args["submoduleListVar"]),
            to_output1=pulumi.Output.from_input(args["submoduleFilterVariable"])
).apply(lambda resolved_outputs: create_submodule_res_with_apply_filter(        {k: v for k, v in resolved_outputs['to_output'] if resolved_outputs["'to_output1'"] == 1}))

        self.register_outputs()
