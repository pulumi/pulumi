import asyncio
import binascii
import os
from pulumi import ComponentResource
from pulumi.dynamic import Resource, ResourceProvider, CreateResult


class RandomResourceProvider(ResourceProvider):
    def create(self, props):
        val = binascii.b2a_hex(os.urandom(15)).decode("ascii")
        return CreateResult(val, {"val": val})


class Random(Resource):
    val: str

    def __init__(self, name, opts=None):
        super().__init__(RandomResourceProvider(), name, {"val": ""}, opts)

class RandomComponent(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("component:RandomComponent", name, {}, opts)

    async def initialize(self, name, t, props, opts):
        # Simulate some async initialization work
        await asyncio.sleep(1)
        # Don't explicitly set the parent resource, it will be set automatically
        self.r = Random(name + "-random")
        self.value = self.r.val

r = RandomComponent("foo")
