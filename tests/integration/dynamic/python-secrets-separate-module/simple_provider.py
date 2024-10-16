# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi.dynamic import CreateResult, Resource, ResourceProvider, Config


class SimpleProviderWithConfig(ResourceProvider):
    def create(self, props, config: Config = None):
        password = config.require("password")
        # This simulates using this as a credential to talk to an external system.
        return CreateResult("0", { "authenticated": "200" if password == "s3cret" else "401" })


class SimpleResourceWithConfig(Resource):
    authenticated: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProviderWithConfig(), name, { "authenticated": None })


class SimpleProvider(ResourceProvider):
    def create(self, props):
        return CreateResult("0", {"authenticated": "304"})


class SimpleResource(Resource):
    authenticated: pulumi.Output[str]

    def __init__(self, name):
        super().__init__(SimpleProvider(), name, {"authenticated": None})
