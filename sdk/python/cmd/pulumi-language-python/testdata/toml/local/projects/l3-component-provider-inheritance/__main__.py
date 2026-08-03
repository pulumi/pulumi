import pulumi
from local import Local
import pulumi_simple as simple

explicit = simple.Provider("explicit")
with_providers = Local("withProviders", opts = pulumi.ResourceOptions(providers={
            "simple": explicit,
        }))
pulumi.export("result", with_providers.result)
