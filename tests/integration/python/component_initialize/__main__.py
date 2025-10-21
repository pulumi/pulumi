import asyncio
import binascii
import os
import pulumi
from pulumi.dynamic import Resource, ResourceProvider, CreateResult


class RandomResourceProvider(ResourceProvider):
    def create(self, props):
        val = binascii.b2a_hex(os.urandom(15)).decode("ascii")
        return CreateResult(val, {"val": val})


class Random(Resource):
    val: str

    def __init__(self, name, opts=None):
        super().__init__(RandomResourceProvider(), name, {"val": ""}, opts)

class RandomComponent(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("component:RandomComponent", name, {}, opts)

        # Don't explicitly set the parent resource, it will be set automatically
        self.r = Random(name + "-random")
        self.value = self.r.val
        # It's safe to do nested components/resources
        if name == "foo":
            self.nested = RandomComponent(name + "-nested")

r = RandomComponent("foo")

pulumi.export("randomValue", r.nested.value)