# Copyright 2024, Pulumi Corporation.

from typing import Any, Optional

import pulumi

class Echo(pulumi.CustomResource):
    def __init__(
        self,
        resource_name: str,
        echo: pulumi.Input[Any],
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        props = {
            "echo": echo,
        }
        super().__init__("testprovider:index:Echo", resource_name, props, opts)

    @property
    @pulumi.getter
    def echo(self) -> pulumi.Output[Any]:
        return pulumi.get(self, "echo")
