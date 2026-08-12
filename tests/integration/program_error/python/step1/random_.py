# Copyright 2025, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi


class Random(pulumi.CustomResource):
    def __init__(
        self,
        resource_name: str,
        length: pulumi.Input[int],
        opts: Optional[pulumi.ResourceOptions] = None,
    ):
        props = {
            "length": length,
            "result": None,
        }
        super().__init__("testprovider:index:Random", resource_name, props, opts)

    @property
    @pulumi.getter
    def length(self) -> pulumi.Output[int]:
        return pulumi.get(self, "length")

    @property
    @pulumi.getter
    def result(self) -> pulumi.Output[str]:
        return pulumi.get(self, "result")
