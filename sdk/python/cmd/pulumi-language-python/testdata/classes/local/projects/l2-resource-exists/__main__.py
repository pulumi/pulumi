import pulumi
import pulumi_simple as simple

res = simple.Resource("res", value=True)
exists_result = pulumi.runtime.exists_resource(None, "simple:index:Resource", "checkExists", res.id, {}, pulumi.ResourceOptions())
pulumi.export("existsResult", exists_result)
