import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

# Component with no dependencies (the contrast)
no_depends_on = conformance_component.Simple("noDependsOn", value=True)
# Component with dependsOn
with_depends_on = conformance_component.Simple("withDependsOn", value=True,
opts = pulumi.ResourceOptions(depends_on=[no_depends_on]))
# Make a simple resource so that plugin detection works.
simple_resource = simple.Resource("simpleResource", value=False)
