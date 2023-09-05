# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

from pulumi import ComponentResource, ResourceOptions

from echo import Echo


class MyComponent(ComponentResource):
    def __init__(self, name):
        super().__init__("my:index:MyComponent", name)


# Step 1, create a component and a child resource.
component = MyComponent("component")
res1 = Echo("res1", echo="hello", opts=ResourceOptions(parent=component))
