# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

from pulumi import ComponentResource
from pulumi.dynamic import Resource, ResourceProvider

r = ComponentResource("a:b:c", "b")
rp = ResourceProvider()
r2 = Resource(rp, 'foo', {})
# assert r.id == "foo"