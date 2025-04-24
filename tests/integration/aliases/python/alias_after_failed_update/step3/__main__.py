# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

from pulumi import Alias, ComponentResource, ResourceOptions

from echo import Echo


class MyComponent(ComponentResource):
    def __init__(self, name):
        super().__init__("my:index:MyComponent", name)


# Step 3: Add the alias for the unparented resource, to prevent the Create and Delete.
# There's no need to keep around res2 that will always fail on Create.
component = MyComponent("component")
res1 = Echo("res1", echo="hello", opts=ResourceOptions(aliases=[Alias(parent=component)])) # add alias
