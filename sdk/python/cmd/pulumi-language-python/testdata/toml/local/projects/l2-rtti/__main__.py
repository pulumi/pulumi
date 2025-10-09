import pulumi
import pulumi_simple as simple

res = simple.Resource("res", value=True)
pulumi.export("name", res.pulumi_resource_name)
pulumi.export("type", res.pulumi_resource_type)
