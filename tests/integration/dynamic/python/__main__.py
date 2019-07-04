# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

from pulumi import ComponentResource
from pulumi.dynamic import Resource, ResourceProvider

class MyResourceProvider(ResourceProvider):
    def create(self, props):
        return {'id': "deadbeef", 'outs': props}

rp = MyResourceProvider()
r2 = Resource(rp, 'foo', {})
