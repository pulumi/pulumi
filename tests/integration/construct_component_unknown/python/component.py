# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Optional

import pulumi

class Component(pulumi.ComponentResource):
    def __init__(self, name: str, args: pulumi.Inputs, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("testcomponent:index:Component", name, args, opts, True)
