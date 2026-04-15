import pulumi
import pulumi_component as component
import pulumi_simple as simple

# Make a simple resource to use as a parent
parent = simple.Resource("parent", value=True)
alias_urn = simple.Resource("aliasURN", value=True)
alias_name = simple.Resource("aliasName", value=True)
alias_no_parent = simple.Resource("aliasNoParent", value=True)
alias_parent = simple.Resource("aliasParent", value=True,
opts = pulumi.ResourceOptions(parent=alias_urn))
alias_type = component.Custom("aliasType", value="true")
