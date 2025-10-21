# Copyright 2025-2025, Pulumi Corporation.  All rights reserved.

from pulumi.dynamic import CreateResult, Resource, ResourceProvider


class SimpleProvider(ResourceProvider):
    def create(self, props):
        print("message from provider", flush=True)
        return CreateResult("0", {})


class SimpleResource(Resource):
    def __init__(self, name):
        super().__init__(SimpleProvider(), name, {})


r = SimpleResource("foo")
