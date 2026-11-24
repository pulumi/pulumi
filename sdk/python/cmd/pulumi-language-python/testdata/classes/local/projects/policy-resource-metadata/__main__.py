import pulumi
import pulumi_simple as simple

prov = simple.Provider("prov")
identity = simple.Resource("identity", value=True)
protected = simple.Resource("protected", value=True,
opts = pulumi.ResourceOptions(protect=True))
ignore_changes = simple.Resource("ignoreChanges", value=True,
opts = pulumi.ResourceOptions(ignore_changes=["value"]))
delete_before_replace = simple.Resource("deleteBeforeReplace", value=True,
opts = pulumi.ResourceOptions(delete_before_replace=True))
secret_output = simple.Resource("secretOutput", value=True,
opts = pulumi.ResourceOptions(additional_secret_outputs=["value"]))
custom_timeouts = simple.Resource("customTimeouts", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create="5m")))
explicit_provider = simple.Resource("explicitProvider", value=True,
opts = pulumi.ResourceOptions(provider=prov))
parent = simple.Resource("parent", value=True)
child = simple.Resource("child", value=True,
opts = pulumi.ResourceOptions(parent=parent))
dependency = simple.Resource("dependency", value=True)
depends_on = simple.Resource("dependsOn", value=True,
opts = pulumi.ResourceOptions(depends_on=[dependency]))
property_dependency = simple.Resource("propertyDependency", value=dependency.value)
