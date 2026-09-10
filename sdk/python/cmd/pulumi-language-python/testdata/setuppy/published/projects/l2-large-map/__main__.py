import pulumi
import pulumi_large as large

res = large.Map("res",
    value="leaf",
    depth=300)
pulumi.export("output", res.value)
