import pulumi
import pulumi_simple as simple

res1 = simple.Resource("res1", value=True)
pulumi.export("name", res1.pulumi_resource_name)
pulumi.export("type", res1.pulumi_resource_type)
