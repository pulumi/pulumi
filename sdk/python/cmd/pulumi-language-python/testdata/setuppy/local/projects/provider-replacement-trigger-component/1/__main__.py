import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

res = conformance_component.Simple("res", value=True,
opts = pulumi.ResourceOptions(replacement_trigger="trigger-value-updated"))
simple_resource = simple.Resource("simpleResource", value=False)
