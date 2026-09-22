import pulumi
import pulumi_fail_on_create as fail_on_create
import pulumi_read as read

failing = fail_on_create.Resource("failing", value=False)
res = read.Resource.get("res", "existing-id", lookup="existing-key",
opts = pulumi.ResourceOptions(depends_on=[failing]))
pulumi.export("readValue", res.value)
