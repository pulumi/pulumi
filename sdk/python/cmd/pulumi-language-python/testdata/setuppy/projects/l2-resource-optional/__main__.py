import pulumi
import pulumi_simple as simple
import pulumi_simple_optional as simple_optional

res_a = simple.Resource("resA", value=True)
res_b = simple_optional.Resource("resB", value=res_a.value)
res_c = simple_optional.Resource("resC",
    value=res_b.value,
    text=None)
