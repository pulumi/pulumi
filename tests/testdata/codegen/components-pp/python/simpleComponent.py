import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_random as random

class SimpleComponent(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:SimpleComponent", name, {}, opts)

        first_password = random.RandomPassword(f"{name}-firstPassword",
            length=16,
            special=True,
            opts = pulumi.ResourceOptions(parent=self))

        second_password = random.RandomPassword(f"{name}-secondPassword",
            length=16,
            special=True,
            opts = pulumi.ResourceOptions(parent=self))

        self.register_outputs()
