import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_simple as simple

simple_v2 = simple.Provider("simpleV2")
with_v2 = conformance_component.Simple("withV2", value=True,
opts = pulumi.ResourceOptions(providers={
        "simple": simple_v2,
    },
    version="2.0.0"))
with_v26 = conformance_component.Simple("withV26", value=False,
opts = pulumi.ResourceOptions(providers={
        "simple": simple_v2,
    }))
with_default = conformance_component.Simple("withDefault", value=True,
opts = pulumi.ResourceOptions(providers={
        "simple": simple_v2,
    }))
