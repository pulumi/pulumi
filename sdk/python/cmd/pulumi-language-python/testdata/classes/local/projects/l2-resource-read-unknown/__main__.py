import pulumi
import pulumi_read as read

src = read.Resource("src", value=True)
res = read.Resource.get("res", src.id, lookup="existing-key")
pulumi.export("resourceUrn", res.urn)
pulumi.export("resourceId", res.id)
pulumi.export("lookup", res.lookup)
pulumi.export("value", res.value)
