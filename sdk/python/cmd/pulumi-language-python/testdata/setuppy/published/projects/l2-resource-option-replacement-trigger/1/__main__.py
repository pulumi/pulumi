import pulumi
import pulumi_output as output
import pulumi_simple as simple

replacement_trigger = simple.Resource("replacementTrigger", value=True,
opts = pulumi.ResourceOptions(replacement_trigger="test2"))
unknown = output.Resource("unknown", value=2)
unknown_replacement_trigger = simple.Resource("unknownReplacementTrigger", value=True,
opts = pulumi.ResourceOptions(replacement_trigger=unknown.output))
not_replacement_trigger = simple.Resource("notReplacementTrigger", value=True)
secret_replacement_trigger = simple.Resource("secretReplacementTrigger", value=True,
opts = pulumi.ResourceOptions(replacement_trigger=pulumi.Output.secret([
        3,
        2,
        1,
    ])))
