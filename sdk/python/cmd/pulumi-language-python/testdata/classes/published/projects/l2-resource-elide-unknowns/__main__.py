import pulumi
import pulumi_output as output
import pulumi_simple as simple

# This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
# can still access that field as an output.
prov = output.Provider("prov", elide_unknowns=True)
unknown = output.Resource("unknown", value=1,
opts = pulumi.ResourceOptions(provider=prov))
# Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
res = simple.Resource("res", value=unknown.output.apply(lambda output: output == "hello"))
pulumi.export("out", unknown.output)
