import pulumi
import pulumi_simple as simple

res1 = simple.Resource("res1", value=True)
local_var = res1.value
res2 = simple.Resource("res2", value=local_var)
pulumi.export("out", res2.value)
