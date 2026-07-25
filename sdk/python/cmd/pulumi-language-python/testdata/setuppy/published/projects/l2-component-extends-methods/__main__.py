import pulumi
import pulumi_inherit as inherit

derived = inherit.Derived("derived",
    message="hi",
    scale=1)
pulumi.export("status", derived.get_status().apply(lambda call: call.status))
