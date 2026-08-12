import pulumi
import pulumi_config_enum as config_enum

prov = config_enum.Provider("prov",
    a_string="hello",
    a_enum=config_enum.MyEnum.TWO)
# Reference the provider's outputs - including the enum - from another resource.
res = config_enum.Resource("res",
    the_string=prov.a_string,
    the_enum=prov.a_enum.apply(lambda x: config_enum.MyEnum(x)),
    opts = pulumi.ResourceOptions(provider=prov))
