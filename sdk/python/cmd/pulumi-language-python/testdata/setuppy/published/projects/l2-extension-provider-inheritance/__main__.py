import pulumi
import pulumi_extbase as extbase
import pulumi_myext as myext

prov = extbase.Provider("prov")
parent = extbase.Base("parent", opts = pulumi.ResourceOptions(provider=prov))
# This should inherit prov from parent, because prov is the base provider of myext.
child = myext.Greeting("child", opts = pulumi.ResourceOptions(parent=parent))
