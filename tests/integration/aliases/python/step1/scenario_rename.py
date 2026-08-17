# Copyright 2026, Pulumi Corporation.  All rights reserved.

from pulumi import Alias, ComponentResource, ResourceOptions


class Resource1(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:Resource", name, None, opts)


# Scenario #1 - rename a resource
res1 = Resource1("res1")
