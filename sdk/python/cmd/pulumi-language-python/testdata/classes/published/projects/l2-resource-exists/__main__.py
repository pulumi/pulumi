import pulumi
import pulumi_exists as exists

res = exists.Resource("res", value=True)
exists_result = pulumi.runtime.exists_resource(None, "exists:index:Resource", res.id, {}, pulumi.ResourceOptions())
exists_with_state = pulumi.runtime.exists_resource(None, "exists:index:Resource", res.id, {
    "value": True,
}, pulumi.ResourceOptions())
missing_with_state = pulumi.runtime.exists_resource(None, "exists:index:Resource", res.id, {
    "value": False,
}, pulumi.ResourceOptions())
pulumi.export("existsResult", exists_result)
pulumi.export("existsWithState", exists_with_state)
pulumi.export("missingWithState", missing_with_state)
