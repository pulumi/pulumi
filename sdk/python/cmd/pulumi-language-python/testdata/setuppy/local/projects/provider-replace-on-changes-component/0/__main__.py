import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

with_replace_on_changes = conformance_component.Simple(
    "withReplaceOnChanges",
    value=True,
    opts=pulumi.ResourceOptions(replace_on_changes=["value"]),
)
without_replace_on_changes = conformance_component.Simple("withoutReplaceOnChanges", value=True)
simple_resource = simple.Resource("simpleResource", value=False)
