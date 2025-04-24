import pulumi
import pulumi_unknown as unknown

data = unknown.index.get_data(input="hello")
values = unknown.eks.module_values()
pulumi.export("content", data["content"])
