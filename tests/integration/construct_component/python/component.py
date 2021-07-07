# Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

from typing import Any, Optional

import pulumi

class Component(pulumi.ComponentResource):
    def __init__(self, name: str, echo: pulumi.Input[Any], opts: Optional[pulumi.ResourceOptions] = None):
        props = dict()
        props["echo"] = echo
        props["child_id"] = None
        props["secret"] = None
        super().__init__("testcomponent:index:Component", name, props, opts, True)

    @property
    @pulumi.getter(name="echo")
    def echo(self) -> pulumi.Output[str]:
        return pulumi.get(self, "echo")

    @property
    @pulumi.getter(name="childId")
    def child_id(self) -> pulumi.Output[str]:
        return pulumi.get(self, "child_id")
