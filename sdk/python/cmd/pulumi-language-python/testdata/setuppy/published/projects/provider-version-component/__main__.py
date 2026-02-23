import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

with_v22 = conformance_component.Simple("withV22", value=True)
with_default = conformance_component.Simple("withDefault", value=True)
# Ensure the simple plugin is discoverable for this conformance run.
simple_resource = simple.Resource("simpleResource", value=False)
