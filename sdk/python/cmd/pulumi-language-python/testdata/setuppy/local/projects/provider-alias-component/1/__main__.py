import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

# Make a simple resource to use as a parent
parent = simple.Resource("parent", value=True)
# parent "res" to a new parent and alias it so it doesn't recreate.
res = conformance_component.Simple("res", value=True,
opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(parent=(None if True else ...))],
    parent=parent))
# Make a simple resource so that plugin detection works.
simple_resource = simple.Resource("simpleResource", value=False)
