# Copyright 2016-2022, Pulumi Corporation.  All rights reserved.

import copy

from pulumi import Alias, ComponentResource, ResourceOptions


class Resource(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:Resource", name, None, opts)

# Scenario #6 - Nested parents changing types
class ComponentSix(ComponentResource):
    def __init__(self, name, opts=None):
        # Add an alias that references the old type of this resource...
        aliases = [
            Alias(type_=f"my:module:ComponentSix-v{i}")
            for i in range(0, 100)
        ]

        # ..and then make the super call with the new type of this resource and the added alias.
        opts_copy = copy.copy(opts)
        opts_copy.aliases = aliases

        super().__init__("my:module:ComponentSix-v100", name, None, opts_copy)
        resource = Resource("otherchild", ResourceOptions(parent=self))

class ComponentSixParent(ComponentResource):
    def __init__(self, name, opts=None):
        # Add an alias that references the old type of this resource...
        aliases = [
            Alias(type_=f"my:module:ComponentSixParent-v{i}")
            for i in range(0, 10)
        ]

        # ..and then make the super call with the new type of this resource and the added alias.
        opts = ResourceOptions(aliases=aliases)

        super().__init__("my:module:ComponentSixParent-v10", name, None, opts)
        child = ComponentSix("child", ResourceOptions(parent=self))

comp6 = ComponentSixParent("comp6")