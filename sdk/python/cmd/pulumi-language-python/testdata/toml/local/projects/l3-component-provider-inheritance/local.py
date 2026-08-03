import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_component as component

class Local(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:Local", name, {}, opts)

        # No provider options here: the providers map must be inherited from the
        # enclosing local component and flow through the remote component's
        # registration into its construct call.
        mlc = component.ComponentForeignChild(f"{name}-mlc", value=True,
        opts = pulumi.ResourceOptions(parent=self))

        self.result = mlc.value
        self.register_outputs({
            'result': mlc.value
        })