import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_config as config

class ProviderComponentArgs(TypedDict, total=False):
    text: Input[str]

class ProviderComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: ProviderComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:ProviderComponent", name, args, opts)

        prov = config.Provider(f"{name}-prov", name="my config",
        opts = pulumi.ResourceOptions(parent=self))

        res = config.Resource(f"{name}-res", text=args["text"],
        opts = pulumi.ResourceOptions(parent=self,
            provider=prov))

        self.result = res.text
        self.register_outputs({
            'result': res.text
        })