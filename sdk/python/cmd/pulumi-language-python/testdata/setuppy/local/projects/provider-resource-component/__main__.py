import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

res = conformance_component.Simple("res", value=True)
# Make a simple resource so that plugin detection works.
simple_resource = simple.Resource("simpleResource", value=False)
