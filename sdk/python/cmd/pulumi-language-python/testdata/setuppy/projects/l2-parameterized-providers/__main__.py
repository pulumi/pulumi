import pulumi
import pulumi_parameterized as parameterized

res = parameterized.Resource("res", value="hello world")
