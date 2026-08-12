import pulumi
import pulumi_simple as simple

prov = simple.Provider("prov")
res = simple.Resource("res", value=True,
opts = pulumi.ResourceOptions(provider=prov))
