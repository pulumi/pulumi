import pulumi
import pulumi_simple as simple

res1 = simple.Resource("res1", value=True)
res2 = simple.Resource("res2", value=res1.value.apply(lambda value: not value))
