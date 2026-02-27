import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

target = conformance_component.Simple("target", value=True)
replace_with = conformance_component.Simple("replaceWith", value=True,
opts = pulumi.ResourceOptions(replace_with=[target]))
not_replace_with = conformance_component.Simple("notReplaceWith", value=True)
# Ensure the simple plugin is discoverable for this conformance run.
simple_resource = simple.Resource("simpleResource", value=False)
