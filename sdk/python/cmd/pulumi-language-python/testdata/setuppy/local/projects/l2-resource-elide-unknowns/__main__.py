import pulumi
import pulumi_output as output
import pulumi_simple as simple

# This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
# can still access that field as an output.
prov = output.Provider("prov", elide_unknowns=True)
unknown = output.Resource("unknown", value=1,
opts = pulumi.ResourceOptions(provider=prov))
complex = output.ComplexResource("complex", value=1,
opts = pulumi.ResourceOptions(provider=prov))
# Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
res = simple.Resource("res", value=unknown.output.apply(lambda output: output == "hello"))
res_array = simple.Resource("resArray", value=complex.output_array.apply(lambda output_array: output_array[0] == "hello"))
res_map = simple.Resource("resMap", value=complex.output_map.apply(lambda output_map: output_map["x"] == "hello"))
res_object = simple.Resource("resObject", value=complex.output_object.apply(lambda output_object: output_object.output == "hello"))
pulumi.export("out", unknown.output)
pulumi.export("outArray", complex.output_array[0])
pulumi.export("outMap", complex.output_map["x"])
pulumi.export("outObject", complex.output_object.output)
