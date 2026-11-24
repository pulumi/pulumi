import pulumi
import pulumi_read as read

res = read.Resource.get("res", "existing-id", lookup="existing-key")
pulumi.export("resourceId", res.id)
pulumi.export("resourceUrn", res.urn)
pulumi.export("lookup", res.lookup)
pulumi.export("value", res.value)
