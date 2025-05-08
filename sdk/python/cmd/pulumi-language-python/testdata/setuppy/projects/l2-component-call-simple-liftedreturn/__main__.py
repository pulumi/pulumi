import pulumi
import pulumi_componentreturnscalar as componentreturnscalar

component1 = componentreturnscalar.ComponentCallable("component1", value="bar")
pulumi.export("from_identity", component1.identity())
pulumi.export("from_prefixed", component1.prefixed(prefix="foo-"))
