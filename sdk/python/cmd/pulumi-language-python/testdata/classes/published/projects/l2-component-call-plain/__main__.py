import pulumi
import pulumi_configurer as configurer

configurer_1 = configurer.Configurer("configurer", provider_config="propagated")
custom_from_plain_provider = configurer.Custom("customFromPlainProvider", value="from-plain-provider",
opts = pulumi.ResourceOptions(provider=configurer_1.plain_provider()))
custom_from_nested_plain_provider = configurer.Custom("customFromNestedPlainProvider", value="from-nested-plain-provider",
opts = pulumi.ResourceOptions(provider=configurer_1.nested_plain_provider().provider))
pulumi.export("plainValue", configurer_1.plain_value())
pulumi.export("nestedPlainValue", configurer_1.nested_plain_provider().value)
