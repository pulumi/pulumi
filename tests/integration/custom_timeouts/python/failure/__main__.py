# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

from pulumi import ComponentResource, Resource, ResourceOptions
from pulumi.resource import CustomTimeouts

class Resource1(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:Resource", name, None, opts)

# Attempt to create a resource with a CustomTimeout that should fail
res5 = Resource1("res5",
    opts=ResourceOptions(custom_timeouts='asdf')
)
