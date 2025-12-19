import pulumi
import pulumi_simple as simple

parent = simple.Resource("parent", value=True)
alias_urn = simple.Resource("aliasURN", value=True,
opts = pulumi.ResourceOptions(aliases=["urn:pulumi:test::l2-resource-option-alias::simple:index:Resource::aliasURN"],
    parent=parent))
alias_new_name = simple.Resource("aliasNewName", value=True,
opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(name="aliasName")]))
