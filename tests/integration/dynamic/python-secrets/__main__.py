# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi.dynamic import CreateResult, Resource, ResourceProvider


config = pulumi.Config()
password = config.require_secret("password")


class SimpleProvider(ResourceProvider):
    def create(self, props):
        # Need to use `password.get()` to get the underlying value of the secret
        # from within the serialized code. This configuration value is captured
        # during serialization, and not retrieved at runtime.
        #
        # This simulates using this as a credential to talk to an external system.
        return CreateResult("0", { "authenticated": "200" if password.get() == "s3cret" else "401" })


class SimpleResource(Resource):
    authenticated: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProvider(), name, { "authenticated": None })


r = SimpleResource("foo")
pulumi.export("out", r.authenticated)
