# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import asyncio
import pulumi

from pulumi import Output, export, UNKNOWN
from pulumi.dynamic import Resource, ResourceProvider, CreateResult
from pulumi.runtime import is_dry_run

class MyProvider(ResourceProvider):
    def create(self, props):
        return CreateResult("0", props)

class MyResource(Resource):
    foo: Output

    def __init__(self, name, props, opts = None):
        super().__init__(MyProvider(), name, props, opts)

class GetResource(pulumi.Resource):
    foo: Output

    def __init__(self, urn):
        super().__init__("unused", "unused:unused:unused", True, None, None, False, False, urn)

a = MyResource("a", {
    "foo": "foo",
})

async def check_get():
    a_urn = await a.urn.future()
    a_get = GetResource(a_urn)
    a_foo = await a_get.foo.future()
    assert a_foo == "foo"

export("o", check_get())
