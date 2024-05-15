# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi

class Component(pulumi.ComponentResource):
    foo: pulumi.Output[str]

    def __init__(self, name: str, foo: pulumi.Input[str], opts: Optional[pulumi.ResourceOptions] = None):
        props = dict()
        props["foo"] = foo
        super().__init__("testcomponent:index:Component", name, props, opts, True)
