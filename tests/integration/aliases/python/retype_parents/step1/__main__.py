# Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

from pulumi import Alias, ComponentResource, ResourceOptions


class Resource(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:Resource", name, None, opts)

# Scenario #6 - Nested parents changing types
class ComponentSix(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:ComponentSix-v0", name, None, opts)
        resource = Resource("otherchild", ResourceOptions(parent=self))

class ComponentSixParent(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:ComponentSixParent-v0", name, None, opts)
        child = ComponentSix("child", ResourceOptions(parent=self))

comp6 = ComponentSixParent("comp6")
