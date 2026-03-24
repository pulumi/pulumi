# Copyright 2016, Pulumi Corporation.  All rights reserved.

import pulumi

class MyComponent(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("test:index:MyComponent", name, {}, opts)

pulumi.log.debug("A debug message")

c = MyComponent("mycomponent")
