import pulumi
import pulumi_component as component

component1 = component.ComponentCallable("component1", value="bar")
pulumi.export("from_identity", component1.identity().result)
pulumi.export("from_prefixed", component1.prefixed(prefix="foo-").result)
