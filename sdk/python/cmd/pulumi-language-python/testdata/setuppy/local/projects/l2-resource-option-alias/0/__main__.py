import pulumi
import pulumi_simple as simple

# Make a simple resource to use as a parent
parent = simple.Resource("parent", value=True)
alias_urn = simple.Resource("aliasURN", value=True)
