import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

simple_v2 = simple.Provider("simpleV2")
with_v22 = conformance_component.Simple("withV22", value=True,
opts = pulumi.ResourceOptions(providers={
        "simple": simple_v2,
    }))
with_default = conformance_component.Simple("withDefault", value=True,
opts = pulumi.ResourceOptions(providers={
        "simple": simple_v2,
    }))
