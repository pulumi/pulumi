# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import pulumi


class FooResource(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        alias_opts = pulumi.ResourceOptions(aliases=[pulumi.Alias(type_="my:module:FooResource")])
        opts = pulumi.ResourceOptions.merge(opts, alias_opts)
        super().__init__("my:module:FooResourceNew", name, None, opts)


class ComponentResource(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:ComponentResource", name, None, opts)
        FooResource("child", pulumi.ResourceOptions(parent=self))


ComponentResource("comp")
