import pulumi
import pulumi_extbase as extbase
import pulumi_myext as myext

prov = extbase.Provider("prov")
parent = extbase.Base("parent", opts = pulumi.ResourceOptions(provider=prov))
# This should use prov from parent, because prov is the base provider of myext.
greeting = myext.greet_output(name="World", opts=pulumi.InvokeOutputOptions(parent=parent))
pulumi.export("greeting", greeting.greeting)
