import pulumi
import pulumi_simple as simple

parent = simple.Resource("parent", value=True)
alias_urn = simple.Resource("aliasURN", value=True,
opts = pulumi.ResourceOptions(aliases=["urn:pulumi:test::l2-resource-option-alias::simple:index:Resource::aliasURN"],
    parent=parent))
alias_new_name = simple.Resource("aliasNewName", value=True,
opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(name="aliasName")]))
alias_no_parent = simple.Resource("aliasNoParent", value=True,
opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(parent=(None if True else ...))],
    parent=parent))
alias_parent = simple.Resource("aliasParent", value=True,
opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(parent=alias_urn)],
    parent=parent))
