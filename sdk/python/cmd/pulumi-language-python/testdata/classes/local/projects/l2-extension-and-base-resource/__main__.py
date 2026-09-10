import pulumi
import pulumi_extbase as extbase
import pulumi_myext as myext

greeting = myext.Greeting("greeting")
base = extbase.Base("base")
pulumi.export("parameterValue", greeting.parameter_value)
pulumi.export("baseValue", base.base_value)
