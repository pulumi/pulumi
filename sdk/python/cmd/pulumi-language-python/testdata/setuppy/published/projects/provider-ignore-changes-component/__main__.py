import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

with_ignore_changes = conformance_component.Simple("withIgnoreChanges", value=True,
    opts = pulumi.ResourceOptions(ignore_changes=["value"]))
without_ignore_changes = conformance_component.Simple("withoutIgnoreChanges", value=True)
# Make a simple resource so that plugin detection works.
simple_resource = simple.Resource("simpleResource", value=False)
