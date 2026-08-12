import pulumi
import pulumi_large as large

res = large.String("res", value="hello world")
pulumi.export("output", res.value)
