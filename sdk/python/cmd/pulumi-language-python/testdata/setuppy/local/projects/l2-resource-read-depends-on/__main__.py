import pulumi
import pulumi_read as read
import pulumi_simple as simple

src = simple.Resource("src", value=True)
res = read.Resource.get("res", "existing-id", lookup="existing-key",
opts = pulumi.ResourceOptions(depends_on=[src]))
