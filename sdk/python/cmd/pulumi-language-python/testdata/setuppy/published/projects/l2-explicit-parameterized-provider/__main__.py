import pulumi
import pulumi_goodbye as goodbye

prov = goodbye.Provider("prov", text="World")
# The resource name is based on the parameter value
res = goodbye.Goodbye("res", opts = pulumi.ResourceOptions(provider=prov))
pulumi.export("parameterValue", res.parameter_value)
