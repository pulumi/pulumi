import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_config as config
import pulumi_multi_argument_invoke as multi_argument_invoke

class InvokeComponent(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:InvokeComponent", name, {}, opts)

        # A multi-argument invoke passes its arguments positionally and omits the ones the program leaves
        # out, so parenting it must not displace the options bag into an argument slot.
        greeting = multi_argument_invoke.multi_argument_invoke_output("hello", opts=pulumi.InvokeOutputOptions(parent=self))

        provider_config = config.get_config_output(text=greeting.result, opts=pulumi.InvokeOutputOptions(parent=self))

        self.result = provider_config.text
        self.register_outputs({
            'result': provider_config.text
        })