import pulumi
import pulumi_extbase as extbase
import pulumi_myext as myext

prov = extbase.Provider("prov")
greeting = myext.Greeting("greeting", opts = pulumi.ResourceOptions(provider=prov))
base = extbase.Base("base", opts = pulumi.ResourceOptions(provider=prov))
pulumi.export("parameterValue", greeting.parameter_value)
pulumi.export("baseValue", base.base_value)
