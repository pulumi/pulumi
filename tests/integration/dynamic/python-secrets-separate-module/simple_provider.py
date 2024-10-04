# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi.dynamic import CreateResult, Resource, ResourceProvider

config = pulumi.Config()
password = config.require_secret("password")


class SimpleProvider(ResourceProvider):
    def create(self, props):
        # This simulates using this as a credential to talk to an external system.
        return CreateResult("0", { "authenticated": password.apply(lambda p: "200" if p == "s3cret" else "401" )})


class SimpleResource(Resource):
    authenticated: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProvider(), name, { "authenticated": None })
