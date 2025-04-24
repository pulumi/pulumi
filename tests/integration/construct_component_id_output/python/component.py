# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

from typing import Any, Optional

import pulumi

class Component(pulumi.ComponentResource):
    def __init__(self, name: str, id: pulumi.Input[str], opts: Optional[pulumi.ResourceOptions] = None):
        props = dict()
        props["id"] = id
        super().__init__("testcomponent:index:Component", name, props, opts, True)

    @property
    @pulumi.getter
    def id(self) -> pulumi.Output[str]:
        return pulumi.get(self, "id")