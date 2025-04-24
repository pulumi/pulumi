# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

from pulumi import ComponentResource, ResourceOptions

from echo import Echo
from fails_on_create import FailsOnCreate


class MyComponent(ComponentResource):
    def __init__(self, name):
        super().__init__("my:index:MyComponent", name)


# Step 2:
#   1. Unparent the resource so the engine will want to do a Create and Delete.
#   2. Add a new resource that always fails on Create, such that the Delete of res1 will not happen.
component = MyComponent("component")
res1 = Echo("res1", echo="hello") # unparent
res2 = FailsOnCreate("res2")
