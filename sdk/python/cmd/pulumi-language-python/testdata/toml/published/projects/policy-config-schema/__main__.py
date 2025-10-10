import pulumi
import pulumi_simple as simple

res_y = simple.Resource("resY", value=True)
res_n = simple.Resource("resN", value=False)
