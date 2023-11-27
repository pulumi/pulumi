# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

# Regression test for https://github.com/pulumi/pulumi/issues/13551
import pulumi

class A(pulumi.ComponentResource):
    def __init__(self, name: str, opts=None):
        super().__init__("my:modules:A", name, {}, opts)
        a_1 = pulumi.CustomResource("my:module:Child-1", "a-child-1", opts=pulumi.ResourceOptions(parent=self, depends_on=[self]))
        pulumi.CustomResource("my:module:Child-2", "a-child-2", props={"transitive_urn": a_1.urn} ,opts=pulumi.ResourceOptions(parent=self))

A("a")
