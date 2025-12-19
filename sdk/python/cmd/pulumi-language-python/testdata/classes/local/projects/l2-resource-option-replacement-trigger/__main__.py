import pulumi
import pulumi_simple as simple

replacement_trigger = simple.Resource("replacementTrigger", value=True,
opts = pulumi.ResourceOptions(replacement_trigger="test"))
not_replacement_trigger = simple.Resource("notReplacementTrigger", value=True)
